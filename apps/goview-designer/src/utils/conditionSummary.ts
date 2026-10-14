const propertyLabels: Record<string, string> = {
  dataset: '数据值（dataset）', x: '横向位置（x）', y: '纵向位置（y）',
  w: '宽度（w）', h: '高度（h）'
}

export function conditionSummary(rule: any, components: any[] = []): string {
  const property = (id: string, title: string, key: string) => {
    const component = components.find(item => item.id === id)
    const name = component?.chartConfig?.title || title || id || '未选择组件'
    return `${name} · ${propertyLabels[key] || key || '未选择属性'}`
  }
  const left = property(rule.id1, rule.title1, rule.content1)
  const right = rule.tiaojianleixingType === '组件联动'
    ? property(rule.id2, rule.title2, rule.content2)
    : String(rule.value ?? '未设置值')
  return `${left} ${rule.fu || '未选择运算符'} ${right}`
}
