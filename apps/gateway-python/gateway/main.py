from __future__ import annotations

import argparse
import asyncio
import base64
import csv
import hashlib
import hmac
import io
import json
import mimetypes
import os
import platform
import secrets
import socket
import subprocess
import sys
import time
import zipfile
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Any

from fastapi import FastAPI, File, Form, HTTPException, Request, UploadFile, WebSocket, WebSocketDisconnect
from fastapi.responses import FileResponse, HTMLResponse, JSONResponse, RedirectResponse, Response, StreamingResponse

from .collectors import read_point
from .configuration import ConfigStore, ConfigurationError, iter_points
from .runtime import GatewayRuntime

LARGE_CONFIG_POINT_THRESHOLD = 50_000
MAX_CONFIG_POINT_WINDOW = 1_000


def create_app(config_path: str | Path, static_dir: str | Path | None = None) -> FastAPI:
    store = ConfigStore(config_path)
    runtime = GatewayRuntime(store)
    assets = resolve_static_dir(static_dir)
    secret = _session_secret(store.path.parent)

    @asynccontextmanager
    async def lifespan(_app: FastAPI):
        await runtime.start()
        try:
            yield
        finally:
            await runtime.stop()

    app = FastAPI(title="微控 Python 网关", version="0.1.0", lifespan=lifespan)
    app.state.store = store
    app.state.runtime = runtime

    @app.middleware("http")
    async def authentication(request: Request, call_next):
        path = request.url.path
        public = path in {"/login", "/api/healthz", "/brand-logo.png"} or path.startswith("/assets/")
        if public or _current_user(request, secret):
            return await call_next(request)
        if path.startswith("/api/"):
            return JSONResponse({"error": "未登录或会话已失效"}, status_code=401)
        return RedirectResponse("/login", status_code=303)

    @app.get("/api/healthz")
    async def healthz():
        return {"ok": True, "backend": "python", "gatewayKey": runtime.state.gateway_key, "uptimeSeconds": runtime.state.summary(False)["uptimeSeconds"]}

    @app.get("/login", response_class=HTMLResponse)
    async def login_page(request: Request):
        if _current_user(request, secret):
            return RedirectResponse("/", status_code=303)
        return HTMLResponse(LOGIN_HTML)

    @app.post("/login")
    async def login(username: str = Form(""), password: str = Form("")):
        user = _authenticate(store.get(), username, password)
        if not user:
            return HTMLResponse(LOGIN_HTML.replace("<!--ERROR-->", '<p class="error">账号或密码错误</p>'), status_code=401)
        response = RedirectResponse("/", status_code=303)
        response.set_cookie("wk_session", _sign_session(username, secret), httponly=True, samesite="strict", max_age=8 * 3600)
        return response

    @app.post("/logout")
    async def logout():
        response = RedirectResponse("/login", status_code=303)
        response.delete_cookie("wk_session")
        return response

    @app.get("/api/session")
    async def session(request: Request):
        username = _current_user(request, secret)
        return {"username": username, "role": _role_for(store.get(), username), "permissions": ["*"]}

    @app.get("/api/config")
    async def get_config():
        content = json.dumps(store.get(redact=True), ensure_ascii=False, indent=2)
        return JSONResponse({"content": content}, headers={"ETag": _config_revision(store.path)})

    @app.get("/api/config/bootstrap")
    async def get_config_bootstrap():
        compact, point_counts, total_points = store.compact(redact=True)
        return JSONResponse({
            "content": json.dumps(compact, ensure_ascii=False, separators=(",", ":")),
            "pointCounts": point_counts,
            "totalPoints": total_points,
            "largeMode": total_points > LARGE_CONFIG_POINT_THRESHOLD,
        }, headers={"ETag": _config_revision(store.path)})

    @app.get("/api/config/points")
    async def get_config_points(
        deviceKey: str,
        offset: int = 0,
        limit: int = 200,
        search: str = "",
        function: int = 0,
    ):
        offset = max(0, offset)
        limit = max(1, min(MAX_CONFIG_POINT_WINDOW, limit))
        try:
            points, total = store.point_window(deviceKey, offset, limit, search, function)
        except KeyError as exc:
            raise HTTPException(404, "device not found") from exc
        return {"offset": offset, "limit": limit, "total": total, "points": points}

    @app.put("/api/config")
    async def put_config(request: Request):
        try:
            body = await request.json()
            value = json.loads(body["content"]) if isinstance(body, dict) and isinstance(body.get("content"), str) else body
            _restore_masked_secrets(value, store.get())
            store.replace(value)
            await runtime.reload()
            return JSONResponse({"ok": True, "restartRequired": False}, headers={"ETag": _config_revision(store.path)})
        except (ValueError, ConfigurationError, json.JSONDecodeError) as exc:
            raise HTTPException(400, str(exc)) from exc

    @app.put("/api/config/compact")
    async def put_compact_config(request: Request):
        try:
            value = await request.json()
            _restore_masked_secrets(value, store.get())
            store.replace_compact(value)
            await runtime.reload()
            return JSONResponse({"ok": True, "restartRequired": False}, headers={"ETag": _config_revision(store.path)})
        except (ValueError, ConfigurationError, json.JSONDecodeError) as exc:
            raise HTTPException(400, str(exc)) from exc

    @app.get("/api/status")
    async def status(compact: int = 0):
        return runtime.state.summary(include_points=not bool(compact))

    @app.get("/api/point-status")
    async def point_status(request: Request, deviceKey: str = ""):
        metrics = request.query_params.getlist("metric")
        return runtime.state.points(deviceKey, metrics or None)

    @app.post("/api/collect-now")
    async def collect_now():
        return await runtime.collect_once()

    @app.post("/api/points/write")
    async def point_write(request: Request):
        body = await request.json()
        if not body.get("metric") or "value" not in body:
            raise HTTPException(400, "deviceKey、metric 和 value 为必填项")
        try:
            return await runtime.write(str(body.get("deviceKey", "")), str(body["metric"]), body["value"])
        except KeyError as exc:
            raise HTTPException(404, str(exc)) from exc
        except Exception as exc:
            raise HTTPException(502, str(exc)) from exc

    @app.websocket("/api/ws")
    async def websocket(websocket: WebSocket):
        if not _verify_session(websocket.cookies.get("wk_session", ""), secret):
            await websocket.close(code=4401)
            return
        await websocket.accept()
        queue = runtime.state.subscribe()
        try:
            await websocket.send_json(runtime.state.message("snapshot", runtime.state.summary()))
            while True:
                try:
                    message = await asyncio.wait_for(queue.get(), timeout=20)
                    await websocket.send_json(message)
                except asyncio.TimeoutError:
                    await websocket.send_json(runtime.state.message("gateway.status", runtime.state.summary(False)))
        except WebSocketDisconnect:
            pass
        finally:
            runtime.state.unsubscribe(queue)

    @app.get("/api/project/export")
    async def export_project():
        payload = json.dumps(store.get(redact=True), ensure_ascii=False, indent=2).encode("utf-8")
        return Response(payload, media_type="application/json", headers={"Content-Disposition": 'attachment; filename="gateway-project.json"'})

    @app.post("/api/project/import")
    async def import_project(file: UploadFile = File(...)):
        try:
            value = json.loads((await file.read()).decode("utf-8-sig"))
            _restore_masked_secrets(value, store.get())
            store.replace(value)
            await runtime.reload()
            return {"ok": True, "message": "工程导入成功"}
        except (UnicodeDecodeError, ValueError, json.JSONDecodeError) as exc:
            raise HTTPException(400, str(exc)) from exc

    @app.get("/api/activation")
    async def get_activation():
        activation = store.get().get("activation", {})
        file_path = str(activation.get("file") or "activation.local.json")
        path = Path(file_path) if Path(file_path).is_absolute() else store.path.parent / file_path
        return {"file": file_path, "content": path.read_text(encoding="utf-8") if path.exists() else "{}"}

    @app.put("/api/activation")
    async def put_activation(request: Request):
        body = await request.json()
        file_path = str(body.get("file") or "activation.local.json")
        path = Path(file_path) if Path(file_path).is_absolute() else store.path.parent / file_path
        try:
            parsed = json.loads(str(body.get("content") or "{}"))
        except json.JSONDecodeError as exc:
            raise HTTPException(400, f"激活文件不是合法JSON：{exc}") from exc
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(parsed, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        return {"ok": True, "file": file_path}

    @app.post("/api/mqtt/test-publish")
    async def mqtt_test_publish():
        topics = await asyncio.to_thread(runtime.mqtt.publish, {"gateway_test": int(time.time())})
        if not topics:
            raise HTTPException(502, "没有已连接的MQTT通道")
        return {"ok": True, "topic": ", ".join(topics)}

    @app.post("/api/connection/test")
    async def connection_test(request: Request):
        body = await request.json()
        protocol = str(body.get("protocol", ""))
        try:
            if protocol in {"modbus-tcp", "modbus-rtu", "opcua"}:
                candidate = dict(body)
                candidate.update({"metric": "connection-test", "register": int(body.get("register", 0)), "function": int(body.get("function", 3)), "quantity": 1, "dataType": "uint16"})
                await asyncio.to_thread(read_point, candidate)
            else:
                host, port = _address(str(body.get("address", "")), 2404 if protocol == "iec104" else 102)
                await asyncio.to_thread(_tcp_probe, host, port, 5)
            return {"ok": True, "message": "连接成功", "address": body.get("address", "")}
        except Exception as exc:
            raise HTTPException(502, str(exc)) from exc

    @app.post("/api/opcua/browse")
    async def opcua_browse(request: Request):
        body = await request.json()
        try:
            nodes = await asyncio.to_thread(_browse_opcua, body)
            return {"ok": True, "nodes": nodes}
        except Exception as exc:
            raise HTTPException(502, str(exc)) from exc

    @app.post("/api/s7-points/scan")
    async def s7_scan():
        raise HTTPException(501, "S7扫描尚未迁移到Python版本")

    @app.post("/api/s7-connection/test")
    async def s7_connection_test():
        raise HTTPException(501, "S7连接测试尚未迁移到Python版本")

    @app.post("/api/iec61850/browse")
    async def iec61850_browse():
        raise HTTPException(501, "IEC61850 MMS浏览尚未迁移到Python版本")

    @app.post("/api/iec61850/cid/preview")
    async def iec61850_preview():
        raise HTTPException(501, "CID/ICD解析尚未迁移到Python版本")

    @app.post("/api/edge-compute/validate")
    async def edge_validate(request: Request):
        body = await request.json()
        expression = str(body.get("expression", "")).strip()
        variables = body.get("variables") if isinstance(body.get("variables"), dict) else {}
        try:
            value = _safe_calculate(expression, variables)
            return {"ok": True, "result": value}
        except Exception as exc:
            raise HTTPException(400, str(exc)) from exc

    @app.get("/api/security")
    async def get_security():
        security = store.get().get("security", {})
        users = [{key: value for key, value in user.items() if key != "password"} for user in security.get("users", [])]
        return {"users": users, "roles": security.get("roles", []), "permissions": ["status.view", "config.manage", "cloud.manage", "network.manage", "maintenance.run"]}

    @app.put("/api/security")
    async def put_security(request: Request):
        body = await request.json()
        config = store.get()
        old_users = {item.get("username"): item for item in config.get("security", {}).get("users", [])}
        users = []
        for item in body.get("users", []):
            user = dict(item)
            if not user.get("password"):
                user["password"] = old_users.get(user.get("username"), {}).get("password", "123456")
            users.append(user)
        config["security"] = {"users": users, "roles": body.get("roles", [])}
        store.replace(config)
        return {"ok": True}

    @app.get("/api/storage")
    async def storage():
        return _storage_status(store.get().get("offlineCache", {}), store.path.parent / ".runtime" / "spool.jsonl")

    @app.put("/api/storage")
    async def put_storage(request: Request):
        config = store.get()
        config["offlineCache"].update(await request.json())
        store.replace(config)
        return _storage_status(config["offlineCache"], store.path.parent / ".runtime" / "spool.jsonl")

    @app.delete("/api/storage")
    async def clear_storage():
        path = store.path.parent / ".runtime" / "spool.jsonl"
        path.unlink(missing_ok=True)
        return {"ok": True}

    @app.get("/api/history-storage")
    async def history_storage():
        path = runtime._history_path()
        result = _storage_status(store.get().get("historyStorage", {}), path)
        result.update({"bytes": path.stat().st_size if path.exists() else 0, "files": 1 if path.exists() else 0, "historyPath": str(path)})
        return result

    @app.put("/api/history-storage")
    async def put_history_storage(request: Request):
        config = store.get()
        config["historyStorage"].update(await request.json())
        store.replace(config)
        return await history_storage()

    @app.delete("/api/history-storage")
    async def clear_history_storage():
        runtime._history_path().unlink(missing_ok=True)
        return {"ok": True}

    @app.get("/api/history-storage/export")
    async def export_history_storage():
        stream = io.StringIO()
        runtime.export_history(stream)
        return Response(stream.getvalue().encode("utf-8-sig"), media_type="text/csv", headers={"Content-Disposition": 'attachment; filename="gateway-history.csv"'})

    @app.get("/api/network/interfaces")
    async def network_interfaces():
        return {"interfaces": _interfaces()}

    @app.post("/api/network/apply")
    async def network_apply():
        raise HTTPException(501, "网口参数写入尚未迁移到Python版本")

    @app.get("/api/network/wifi")
    async def wifi_status():
        return {"available": False, "connected": False, "message": "Python首版保留接口，WiFi配置迁移中"}

    @app.put("/api/network/wifi")
    async def wifi_save():
        raise HTTPException(501, "WiFi配置迁移中")

    @app.get("/api/network/wifi/scan")
    async def wifi_scan():
        raise HTTPException(501, "WiFi扫描迁移中")

    @app.get("/api/network/cellular")
    async def cellular_status():
        return {"enabled": False, "available": False, "message": "Python首版保留接口，4G拨号管理迁移中"}

    @app.put("/api/network/cellular")
    @app.post("/api/network/cellular")
    async def cellular_save():
        raise HTTPException(501, "4G拨号管理迁移中")

    @app.post("/api/maintenance/ping")
    async def ping(request: Request):
        body = await request.json()
        target = str(body.get("target", "")).strip()
        if not target or any(character not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-:" for character in target):
            raise HTTPException(400, "目标地址格式不正确")
        count = min(10, max(1, int(body.get("count", 4))))
        arguments = ["ping", "-n" if os.name == "nt" else "-c", str(count), target]
        completed = await asyncio.to_thread(subprocess.run, arguments, capture_output=True, text=True, timeout=30)
        return {"ok": completed.returncode == 0, "output": completed.stdout or completed.stderr}

    @app.get("/api/maintenance/audit-log")
    async def audit_log(limit: int = 100):
        logs = runtime.state.summary(False)["errors"][:max(1, min(500, limit))]
        return {"events": [{"timestamp": item["time"], "method": "RUNTIME", "path": "gateway-python", "status": 200, "result": item["level"], "detail": item["message"]} for item in logs]}

    @app.get("/api/packets")
    async def packet_frames():
        return {"frames": [], "backend": "python", "message": "Python首版报文捕获迁移中"}

    @app.get("/api/packets/stream")
    async def packet_stream():
        async def events():
            while True:
                yield ": keepalive\n\n"
                await asyncio.sleep(15)
        return StreamingResponse(events(), media_type="text/event-stream")

    @app.post("/api/maintenance/restart")
    async def restart_service():
        return {"ok": True, "message": "请由systemd或启动脚本重启Python网关服务"}

    @app.post("/api/maintenance/reboot")
    async def reboot_gateway():
        raise HTTPException(501, "首版为安全起见未开放操作系统重启")

    @app.post("/api/maintenance/factory-reset")
    async def factory_reset():
        raise HTTPException(501, "首版为安全起见未开放恢复出厂设置")

    @app.get("/brand-logo.png")
    async def brand_logo():
        candidate = assets / "brand-logo.png" if assets else None
        if candidate and candidate.exists():
            return FileResponse(candidate)
        return Response(status_code=404)

    @app.get("/packet-monitor")
    async def packet_monitor_page():
        candidate = assets.parent / "packet-monitor.html" if assets else None
        if candidate and candidate.exists():
            return FileResponse(candidate)
        return HTMLResponse("<h1>实时报文监控</h1><p>Python首版报文捕获迁移中。</p>")

    @app.get("/{path:path}")
    async def frontend(path: str):
        if not assets:
            return HTMLResponse("<h1>微控 Python 网关</h1><p>Vue构建产物未找到，请执行 package.ps1 或使用 --static-dir 指定目录。</p>")
        candidate = (assets / path).resolve()
        if path and candidate.is_file() and assets.resolve() in candidate.parents:
            return FileResponse(candidate, media_type=mimetypes.guess_type(candidate.name)[0])
        index = assets / "index.html"
        return FileResponse(index) if index.exists() else HTMLResponse("<h1>Vue index.html 不存在</h1>", status_code=404)

    return app


def run() -> None:
    parser = argparse.ArgumentParser(description="微控Python网关")
    parser.add_argument("--config", default="config.local.json")
    parser.add_argument("--listen", default="")
    parser.add_argument("--static-dir", default="")
    parser.add_argument("--check-config", action="store_true")
    args = parser.parse_args()
    store = ConfigStore(args.config)
    if args.check_config:
        print(f"configuration is valid: {store.path}")
        return
    listen = args.listen or str(store.get().get("web", {}).get("listen") or "0.0.0.0:8089")
    host, port = _address(listen, 8089)
    import uvicorn

    uvicorn.run(create_app(args.config, args.static_dir or None), host=host, port=port, log_level="info")


def resolve_static_dir(explicit: str | Path | None) -> Path | None:
    candidates = []
    if explicit:
        candidates.append(Path(explicit))
    candidates.extend([
        Path(__file__).resolve().parents[1] / "frontend",
        Path(__file__).resolve().parents[2] / "gateway-go" / "internal" / "web" / "frontend" / "vue-dist",
    ])
    return next((path for path in candidates if (path / "index.html").exists()), None)


def _session_secret(directory: Path) -> bytes:
    path = directory / ".runtime" / "python-session-secret"
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists():
        path.write_text(secrets.token_hex(32), encoding="ascii")
    return path.read_text(encoding="ascii").strip().encode("ascii")


def _config_revision(path: Path) -> str:
    return '"' + hashlib.sha256(path.read_bytes()).hexdigest()[:24] + '"'


def _sign_session(username: str, secret: bytes) -> str:
    expires = str(int(time.time()) + 8 * 3600)
    message = f"{username}.{expires}".encode()
    signature = hmac.new(secret, message, hashlib.sha256).hexdigest()
    return base64.urlsafe_b64encode(message + b"." + signature.encode()).decode()


def _verify_session(value: str, secret: bytes) -> str:
    try:
        raw = base64.urlsafe_b64decode(value.encode()).decode()
        username, expires, signature = raw.rsplit(".", 2)
        message = f"{username}.{expires}".encode()
        if int(expires) < time.time() or not hmac.compare_digest(signature, hmac.new(secret, message, hashlib.sha256).hexdigest()):
            return ""
        return username
    except (ValueError, UnicodeDecodeError):
        return ""


def _current_user(request: Request, secret: bytes) -> str:
    return _verify_session(request.cookies.get("wk_session", ""), secret)


def _authenticate(config: dict[str, Any], username: str, password: str) -> dict[str, Any] | None:
    users = config.get("security", {}).get("users", [])
    for user in users:
        if user.get("username") == username and user.get("enabled", True) is not False and _verify_password(user, password):
            return user
    return None


def _verify_password(user: dict[str, Any], password: str) -> bool:
    plain = str(user.get("password", ""))
    if plain:
        return hmac.compare_digest(plain, password)
    encoded = str(user.get("passwordHash", ""))
    parts = encoded.split(":")
    if len(parts) == 3 and parts[0] == "sha256":
        actual = hashlib.sha256(f"{parts[1]}:{password}".encode()).hexdigest()
        return hmac.compare_digest(actual, parts[2])
    return False


def _role_for(config: dict[str, Any], username: str) -> str:
    return next((str(item.get("roleKey") or item.get("role") or "admin") for item in config.get("security", {}).get("users", []) if item.get("username") == username), "admin")


def _restore_masked_secrets(value: dict[str, Any], current: dict[str, Any]) -> None:
    for name in ("mqtt", "activation"):
        for key in ("password", "deviceSecret"):
            if value.get(name, {}).get(key) == "********":
                value[name][key] = current.get(name, {}).get(key, "")
    old_devices = {item.get("deviceKey"): item for item in current.get("devices", [])}
    for device in value.get("devices", []):
        if device.get("password") == "********":
            device["password"] = old_devices.get(device.get("deviceKey"), {}).get("password", "")
    if "security" not in value:
        value["security"] = current.get("security", {})


def _address(value: str, default_port: int) -> tuple[str, int]:
    text = value.strip()
    if text.startswith("tcp://"):
        text = text[len("tcp://") :]
    if text.startswith("[") and "]:" in text:
        host, port = text[1:].split("]:", 1)
        return host, int(port)
    if ":" in text:
        host, port = text.rsplit(":", 1)
        return host or "0.0.0.0", int(port)
    return text or "0.0.0.0", default_port


def _tcp_probe(host: str, port: int, timeout: float) -> None:
    with socket.create_connection((host, port), timeout=timeout):
        return


def _browse_opcua(body: dict[str, Any]) -> list[dict[str, Any]]:
    from asyncua.sync import Client

    client = Client(str(body.get("address", "")), timeout=10)
    if body.get("username"):
        client.set_user(str(body["username"]))
        client.set_password(str(body.get("password", "")))
    with client:
        root = client.get_node(str(body.get("nodeId"))) if body.get("nodeId") else client.nodes.objects
        return [{"nodeId": child.nodeid.to_string(), "name": child.read_browse_name().Name, "displayName": child.read_display_name().Text, "hasChildren": bool(child.get_children())} for child in root.get_children()]


def _safe_calculate(expression: str, variables: dict[str, Any]) -> Any:
    import ast
    import math
    import operator

    operators = {ast.Add: operator.add, ast.Sub: operator.sub, ast.Mult: operator.mul, ast.Div: operator.truediv, ast.Mod: operator.mod, ast.Pow: operator.pow,
                 ast.Gt: operator.gt, ast.GtE: operator.ge, ast.Lt: operator.lt, ast.LtE: operator.le, ast.Eq: operator.eq, ast.NotEq: operator.ne,
                 ast.And: lambda a, b: a and b, ast.Or: lambda a, b: a or b}
    functions = {"min": min, "max": max, "abs": abs, "round": round, "floor": math.floor, "ceil": math.ceil}

    def evaluate(node):
        if isinstance(node, ast.Expression): return evaluate(node.body)
        if isinstance(node, ast.Constant): return node.value
        if isinstance(node, ast.Name) and node.id in variables: return variables[node.id]
        if isinstance(node, ast.BinOp) and type(node.op) in operators: return operators[type(node.op)](evaluate(node.left), evaluate(node.right))
        if isinstance(node, ast.BoolOp) and type(node.op) in operators:
            result = evaluate(node.values[0])
            for item in node.values[1:]: result = operators[type(node.op)](result, evaluate(item))
            return result
        if isinstance(node, ast.Compare):
            left = evaluate(node.left)
            return all(operators[type(op)](left if index == 0 else evaluate(node.comparators[index - 1]), evaluate(right)) for index, (op, right) in enumerate(zip(node.ops, node.comparators)))
        if isinstance(node, ast.UnaryOp) and isinstance(node.op, (ast.USub, ast.UAdd, ast.Not)):
            value = evaluate(node.operand)
            return -value if isinstance(node.op, ast.USub) else (+value if isinstance(node.op, ast.UAdd) else not value)
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in functions:
            return functions[node.func.id](*(evaluate(argument) for argument in node.args))
        raise ValueError("表达式包含不允许的语法")

    return evaluate(ast.parse(expression, mode="eval"))


def _storage_status(config: dict[str, Any], path: Path) -> dict[str, Any]:
    root = Path(str(config.get("storagePath") or path.parent))
    active = bool(config.get("enabled")) and root.exists()
    return {
        "configured": bool(config.get("enabled")), "active": active, "maxSizeMB": int(config.get("maxSizeMB", 16)),
        "storagePath": str(root), "cachePath": str(path), "cacheBytes": path.stat().st_size if path.exists() else 0, "devices": [],
    }


def _interfaces() -> list[dict[str, Any]]:
    names = [name for _, name in socket.if_nameindex()] if hasattr(socket, "if_nameindex") else []
    return [{"name": name, "interface": name, "enabled": True} for name in names]


LOGIN_HTML = """<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>微控网关登录</title><style>body{margin:0;background:#f4f7fb;font:14px system-ui;color:#172033;display:grid;place-items:center;height:100vh}.box{width:320px;background:white;padding:28px;border-radius:14px;box-shadow:0 16px 50px #22335518}h1{font-size:20px;margin:0 0 8px}p{color:#748099}label{display:block;margin:16px 0 6px}input{box-sizing:border-box;width:100%;height:40px;border:1px solid #d7e0ec;border-radius:8px;padding:0 12px}button{width:100%;height:40px;border:0;border-radius:8px;background:#1677ff;color:white;margin-top:20px}.error{color:#e5484d}</style></head><body><form class="box" method="post"><h1>微控网关</h1><p>Python Backend Edition</p><!--ERROR--><label>账号</label><input name="username" value="admin" autocomplete="username"><label>密码</label><input name="password" type="password" autocomplete="current-password"><button>登录</button></form></body></html>"""


if __name__ == "__main__":
    run()
