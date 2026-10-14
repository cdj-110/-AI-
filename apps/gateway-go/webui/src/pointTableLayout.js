export function pointTableSchemaSignature(columns) {
    return columns.map(column => `${column.label}\u001f${column.className}`).join('\u001e');
}
export function pointTableColumnMinimum(index, column) {
    if (index === 0 || column.className.includes('point-sheet-select'))
        return 34;
    if (index === 1 || column.className.includes('point-sheet-index'))
        return 42;
    if (column.className.includes('point-name-cell') || column.label === '名称')
        return 240;
    if (column.className.includes('point-metric-cell') || column.label === '标识符')
        return 150;
    if (column.className.includes('point-live') || column.label === '实时值')
        return 110;
    if (column.className.includes('point-sheet-actions') || column.label === '操作')
        return 104;
    if (column.className.includes('point-address') || ['NodeId', '对象引用'].includes(column.label))
        return 220;
    if (['功能码', '数据类型', '点类型'].includes(column.label))
        return 96;
    if (['寄存器', '字节地址', '字节序', '数据集索引'].includes(column.label))
        return 88;
    if (['数量', '区域', 'DB号', 'IOA', 'FC', '倍率', '偏移', '单位', '小数位'].includes(column.label))
        return 72;
    return 72;
}
export function normalizePointTableWidths(measured, columns) {
    return columns.map((column, index) => {
        const width = Number.isFinite(measured[index]) ? Math.round(measured[index]) : 0;
        return Math.max(pointTableColumnMinimum(index, column), width);
    });
}
