export default {
  // 页面标题与标签
  title: 'Modbus Configuration',
  updateModel: 'Update Device Model',
  
  // 表格列标题
  functionName: 'Function Name',
  slaveAddress: 'Slave Address',
  registerType: 'Register Type',
  registerAddress: 'Register Address',
  dataType: 'Data Type',
  registerCount: 'Register Count',
  byteOrder: 'Byte Order',
  description: 'Description',
  
  // 寄存器类型选项
  coil: 'Coil (0x01)',
  discreteInput: 'Discrete Input (0x02)',
  holdingRegister: 'Holding Register (0x03)',
  inputRegister: 'Input Register (0x04)',
  
  // 数据类型选项
  boolean: 'Boolean',
  int8: '8-bit Integer',
  uint8: '8-bit Unsigned Integer',
  int16: '16-bit Integer',
  uint16: '16-bit Unsigned Integer',
  int32: '32-bit Integer',
  uint32: '32-bit Unsigned Integer',
  float32: '32-bit Float',
  float64: '64-bit Float',
  string: 'String',
  
  // 字节序选项
  bigEndian: 'Big Endian (ABCD)',
  littleEndian: 'Little Endian (DCBA)',
  bigEndianWordSwap: 'Big Endian Word Swap (BADC)',
  littleEndianWordSwap: 'Little Endian Word Swap (CDAB)',
  
  // 按钮与操作
  addPoint: 'Add Point',
  editPoint: 'Edit Point',
  deletePoint: 'Delete Point',
  batchImport: 'Batch Import',
  batchExport: 'Batch Export',
  testConnection: 'Test Connection',
  
  // 表单标签
  pointName: 'Point Name',
  pointCode: 'Point Code',
  readInterval: 'Read Interval (ms)',
  writeEnable: 'Write Enable',
  scalingFactor: 'Scaling Factor',
  offset: 'Offset',
  unit: 'Unit',
  
  // 提示信息
  confirmDeletePoint: 'Are you sure you want to delete this point?',
  deletePointSuccess: 'Point deleted successfully',
  savePointSuccess: 'Point saved successfully',
  testConnectionSuccess: 'Connection test successful',
  testConnectionFailed: 'Connection test failed, please check configuration',
  noModbusPoints: 'No Modbus points configured',
  functionUnderDevelopment: 'Modbus configuration under development'
}