export default {
  // 页面标题与标签
  title: 'Modbus配置',
  updateModel: '更新设备模型',
  
  // 表格列标题
  functionName: '功能名称',
  slaveAddress: '从机地址',
  registerType: '寄存器类型',
  registerAddress: '寄存器地址',
  dataType: '数据类型',
  registerCount: '寄存器数量',
  byteOrder: '字节序',
  description: '描述',
  
  // 寄存器类型选项
  coil: '线圈(0x01)',
  discreteInput: '离散输入(0x02)',
  holdingRegister: '保持寄存器(0x03)',
  inputRegister: '输入寄存器(0x04)',
  
  // 数据类型选项
  boolean: '布尔值',
  int8: '8位整数',
  uint8: '8位无符号整数',
  int16: '16位整数',
  uint16: '16位无符号整数',
  int32: '32位整数',
  uint32: '32位无符号整数',
  float32: '32位浮点数',
  float64: '64位浮点数',
  string: '字符串',
  
  // 字节序选项
  bigEndian: '大端序(ABCD)',
  littleEndian: '小端序(DCBA)',
  bigEndianWordSwap: '大端字交换(BADC)',
  littleEndianWordSwap: '小端字交换(CDAB)',
  
  // 按钮与操作
  addPoint: '新增点位',
  editPoint: '编辑点位',
  deletePoint: '删除点位',
  batchImport: '批量导入',
  batchExport: '批量导出',
  testConnection: '测试连接',
  
  // 表单标签
  pointName: '点位名称',
  pointCode: '点位编码',
  readInterval: '读取间隔(ms)',
  writeEnable: '允许写入',
  scalingFactor: '缩放系数',
  offset: '偏移量',
  unit: '单位',
  
  // 提示信息
  confirmDeletePoint: '确定要删除该点位吗？',
  deletePointSuccess: '点位删除成功',
  savePointSuccess: '点位保存成功',
  testConnectionSuccess: '连接测试成功',
  testConnectionFailed: '连接测试失败，请检查配置',
  noModbusPoints: '暂无Modbus点位配置',
  functionUnderDevelopment: 'Modbus配置功能开发中'
}