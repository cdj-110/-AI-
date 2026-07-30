"""Weikong Python gateway backend."""

import asyncio
import contextvars
import faulthandler
import functools
import signal
from typing import Any, Callable

__version__ = "0.1.0"


if not hasattr(asyncio, "to_thread"):
    async def _to_thread(function: Callable[..., Any], /, *args: Any, **kwargs: Any) -> Any:
        """Python 3.8 compatible implementation of asyncio.to_thread."""
        loop = asyncio.get_running_loop()
        context = contextvars.copy_context()
        call = functools.partial(context.run, function, *args, **kwargs)
        return await loop.run_in_executor(None, call)

    asyncio.to_thread = _to_thread  # type: ignore[attr-defined]


try:
    faulthandler.register(signal.SIGUSR1)
except (AttributeError, OSError, RuntimeError):
    pass
