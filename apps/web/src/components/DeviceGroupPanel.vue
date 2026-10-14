<script setup lang="ts">
import { computed, ref } from 'vue';
import { ArrowDown, Delete, Edit, Folder, Grid, InfoFilled, Plus } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { apiRequest } from '../api/request';

export interface DeviceGroupItem { id: string; name: string; _count: { devices: number } }
const props = defineProps<{ items: DeviceGroupItem[]; selected: string; total: number; ungrouped: number; canManage: boolean; loading: boolean }>();
const emit = defineEmits<{ select: [id: string]; changed: [deletedId?: string] }>();
const keyword = ref(''), popoverVisible = ref(false), dialogVisible = ref(false);
const editingId = ref(''), name = ref(''), saving = ref(false), deletingId = ref('');
const current = computed(() => props.items.find(item => item.id === props.selected));
const selectedName = computed(() => !props.selected ? '全部设备' : props.selected === 'ungrouped' ? '未分组' : current.value?.name ?? '全部设备');
const filtered = computed(() => props.items.filter(item => item.name.toLocaleLowerCase().includes(keyword.value.trim().toLocaleLowerCase())));

function select(id: string) {
  emit('select', id);
  popoverVisible.value = false;
}
function edit(group?: DeviceGroupItem) {
  editingId.value = group?.id ?? '';
  name.value = group?.name ?? '';
  popoverVisible.value = false;
  dialogVisible.value = true;
}
async function save() {
  if (saving.value) return;
  const value = name.value.trim();
  if (!value) { ElMessage.warning('请输入分组名称'); return; }
  saving.value = true;
  try {
    await apiRequest({ url: editingId.value ? `/api/device-groups/${editingId.value}` : '/api/device-groups', method: editingId.value ? 'PATCH' : 'POST', data: { name: value } });
    dialogVisible.value = false;
    ElMessage.success(editingId.value ? '分组已重命名' : '分组已创建');
    emit('changed');
  } catch { /* Request layer displays the error; retain the user's input. */ }
  finally { saving.value = false; }
}
async function remove(group: DeviceGroupItem) {
  if (deletingId.value) return;
  deletingId.value = group.id;
  try {
    await ElMessageBox.confirm(`删除“${group.name}”后，只会解除设备与该分组的关联；设备仍保留在其他分组中，不影响采集和连接。`, '删除分组', { type: 'warning', confirmButtonText: '删除分组', cancelButtonText: '取消' });
    await apiRequest({ url: `/api/device-groups/${group.id}`, method: 'DELETE' });
    popoverVisible.value = false;
    ElMessage.success('分组已删除，设备已保留');
    emit('changed', group.id);
  } catch { /* Cancellation and request errors leave the list unchanged. */ }
  finally { deletingId.value = ''; }
}
</script>

<template>
  <div class="device-group-filter">
    <el-popover v-model:visible="popoverVisible" placement="bottom-start" :width="292" trigger="click" :show-arrow="false" popper-class="device-group-popper">
      <template #reference>
        <el-button class="group-trigger" :aria-label="`设备分组：${selectedName}`" aria-haspopup="menu">
          <el-icon><Folder /></el-icon><span>分组：{{ selectedName }}</span><el-icon class="trigger-arrow"><ArrowDown /></el-icon>
        </el-button>
      </template>
      <div v-loading="loading" class="device-group-popover" aria-label="设备分组选择">
        <div class="group-heading">
          <div class="group-title">设备分组<el-tooltip content="设备可加入多个分组；分组可用于成员访问范围，不改变网关关系" placement="top"><el-icon class="group-info"><InfoFilled /></el-icon></el-tooltip></div>
          <el-button v-if="canManage" :icon="Plus" text circle aria-label="新建分组" title="新建分组" @click="edit()" />
        </div>
        <el-input v-model="keyword" size="small" clearable placeholder="搜索分组" aria-label="搜索分组" />
        <nav class="group-list" aria-label="按分组查看设备">
          <button type="button" class="group-item" :class="{ active: !selected }" :aria-current="!selected ? 'true' : undefined" @click="select('')"><el-icon><Grid /></el-icon><span>全部设备</span><small>{{ total }}</small></button>
          <button type="button" class="group-item" :class="{ active: selected === 'ungrouped' }" :aria-current="selected === 'ungrouped' ? 'true' : undefined" @click="select('ungrouped')"><el-icon><Folder /></el-icon><span>未分组</span><small>{{ ungrouped }}</small></button>
          <div class="group-divider" />
          <div v-for="group in filtered" :key="group.id" class="group-row">
            <button type="button" class="group-item" :class="{ active: selected === group.id }" :aria-current="selected === group.id ? 'true' : undefined" :title="group.name" @click="select(group.id)"><el-icon><Folder /></el-icon><span>{{ group.name }}</span><small>{{ group._count.devices }}</small></button>
            <div v-if="canManage" class="row-actions">
              <el-button :icon="Edit" text circle :aria-label="`重命名${group.name}`" :title="`重命名${group.name}`" @click.stop="edit(group)" />
              <el-button :icon="Delete" text circle type="danger" :loading="deletingId === group.id" :aria-label="`删除${group.name}`" :title="`删除${group.name}`" @click.stop="remove(group)" />
            </div>
          </div>
          <p v-if="!filtered.length" class="group-empty">{{ keyword ? '没有匹配的分组' : '暂无自定义分组' }}</p>
        </nav>
      </div>
    </el-popover>
    <el-dialog v-model="dialogVisible" :title="editingId ? '重命名分组' : '新建设备分组'" width="min(420px, 92vw)" :close-on-click-modal="!saving" :close-on-press-escape="!saving" :show-close="!saving">
      <el-form label-position="top" @submit.prevent="save"><el-form-item label="分组名称" required><el-input v-model="name" aria-label="分组名称" maxlength="50" show-word-limit placeholder="例如：一号车间、办公区" :disabled="saving" @keyup.enter="save" /></el-form-item></el-form>
      <template #footer><el-button :disabled="saving" @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.device-group-filter{min-width:0}.group-trigger{max-width:220px}.group-trigger span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.trigger-arrow{margin-left:4px;color:#94a3b8}.device-group-popover{min-height:120px}.group-heading{display:flex;align-items:center;justify-content:space-between;margin-bottom:10px}.group-title{display:flex;align-items:center;gap:6px;font-size:14px;font-weight:600;color:#263b58}.group-info{color:#94a3b8;cursor:help}.group-list{display:grid;gap:3px;margin-top:10px;max-height:320px;overflow:auto}.group-row{display:flex;align-items:center;min-width:0}.group-row>.group-item{flex:1;min-width:0}.group-item{display:flex;align-items:center;gap:9px;width:100%;padding:9px 10px;border:0;border-radius:7px;background:transparent;color:#63748b;text-align:left;cursor:pointer;font:inherit;font-size:13px}.group-item:hover{background:#f5f8fd}.group-item.active{color:#1677ff;background:#edf5ff;font-weight:600}.group-item:focus-visible{outline:2px solid #1677ff;outline-offset:-2px}.group-item span{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.group-item small{font-size:12px;font-variant-numeric:tabular-nums;color:#8a9ab0}.group-item.active small{color:#1677ff}.group-divider{height:1px;margin:6px 8px;background:#edf1f7}.group-empty{margin:14px 8px;color:#91a0b4;font-size:12px;text-align:center}.row-actions{display:flex;flex:0 0 auto;margin-left:3px}.row-actions .el-button{width:28px;height:28px;margin:0}
@media(max-width:560px){.device-group-filter,.group-trigger{width:100%;max-width:none}}
</style>
