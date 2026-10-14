require('reflect-metadata');
const assert = require('node:assert/strict');
const { Reflector } = require('@nestjs/core');
const { PermissionsGuard } = require('../dist/common/guards/permissions.guard');
const { DevicesController } = require('../dist/devices/devices.controller');
const { DeviceGroupsController } = require('../dist/devices/device-groups.controller');
const { ModelTemplatesController } = require('../dist/model-templates/model-templates.controller');
const { AlarmsController } = require('../dist/alarms/alarms.controller');
const { normalizeTenantUserPermissions, DEFAULT_TENANT_USER_PERMISSIONS } = require('../dist/common/user-permissions');

const guard = new PermissionsGuard(new Reflector());
function context(controller, method, permissions) {
  return {
    getHandler: () => controller.prototype[method],
    getClass: () => controller,
    switchToHttp: () => ({ getRequest: () => ({ user: { role: 'TENANT_USER', permissions } }) }),
  };
}
function allowed(controller, method, permission) {
  assert.equal(guard.canActivate(context(controller, method, [permission])), true, `${method} should accept ${permission}`);
  assert.throws(() => guard.canActivate(context(controller, method, ['DASHBOARD_VIEW'])), /没有访问/, `${method} should reject an unrelated permission`);
}

assert.deepEqual(DEFAULT_TENANT_USER_PERMISSIONS, [
  'DASHBOARD_VIEW', 'DEVICE_VIEW', 'MAP_VIEW', 'SCADA_VIEW', 'MODEL_VIEW', 'ALARM_VIEW', 'EVENT_VIEW',
]);
assert.deepEqual(normalizeTenantUserPermissions(['DEVICE_DELETE', 'MODEL_EDIT', 'ALARM_RESOLVE', 'UNKNOWN']), [
  'DEVICE_VIEW', 'DEVICE_DELETE', 'MODEL_VIEW', 'MODEL_EDIT', 'ALARM_VIEW', 'ALARM_RESOLVE',
]);
allowed(DevicesController, 'create', 'DEVICE_CREATE');
allowed(DevicesController, 'update', 'DEVICE_EDIT');
allowed(DevicesController, 'remove', 'DEVICE_DELETE');
allowed(DeviceGroupsController, 'create', 'DEVICE_GROUP_MANAGE');
allowed(ModelTemplatesController, 'create', 'MODEL_CREATE');
allowed(ModelTemplatesController, 'update', 'MODEL_EDIT');
allowed(ModelTemplatesController, 'remove', 'MODEL_DELETE');
allowed(AlarmsController, 'resolve', 'ALARM_RESOLVE');

console.log('PASS default read-only permissions, action dependencies, and menu/page/button route enforcement.');
