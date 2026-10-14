/*
 * @Author: 何彩彩 "18210946822@163.com"
 * @Date: 2026-01-16 10:43:33
 * @LastEditors: 何彩彩 "18210946822@163.com"
 * @LastEditTime: 2026-01-23 14:48:19
 * @FilePath: \web\go-view-master\src\packages\components\BasicShapes\Graph\TextFun\config.ts
 * @Description: 这是默认设置,请设置 customMade，打开koroFileHeader查看配置 进行设置：https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
import {  PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { TextFun } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'
export const option = {
  //这是组件属性的传值通道
  text: "文本内容",
  prefixText: "",
  suffixText: "",
  textSize1: 30,
  color1: "#ffffff",
  bgColor1: "#4FD2DD00",
  fontWeight1: 500,
  letter1: 10,
  lineHeight1: 1.3,
  writing1: "horizontal-tb",
  decoration1: "",
  fontStyle1: "",
  fontFamily1: "黑体",
  
  textSize2: 30,
  color2: "#ffffff",
  bgColor2: "#4FD2DD00",
  fontWeight2: 500,
  letter2: 10,
  lineHeight2: 1.3,
  writing2: "horizontal-tb",
  decoration2: "",
  fontStyle2: "",
  marginRight: 0,
  fontFamily2: "黑体",

  textSize3: 30,
  color3: "#ffffff",
  bgColor3: "#4FD2DD00",
  fontWeight3: 500,
  letter3: 10,
  lineHeight3: 1.3,
  writing3: "horizontal-tb",
  decoration3: "",
  fontStyle3: "",
  marginLeft: 0,
  fontFamily3: "黑体",
}

export default class Config extends PublicConfigClass implements CreateComponentType {
  public key = TextFun.key
  public attr = { ...chartInitConfig, w: 300, h: 50, zIndex: -1 } // 按钮的高度-后来加的  不要可以去掉
  public chartConfig = cloneDeep(TextFun)
  public option = cloneDeep(option)
}
