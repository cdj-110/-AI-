import { createI18n } from 'vue-i18n'
import zhCnCommon from './zh-CN/common'
import zhCnDevice from './zh-CN/device'
import zhCnModbus from './zh-CN/modbus'
import zhCnUserOrganize from './zh-CN/userOrganize'
import zhCnUserAccount from './modules/userAccount/zh-CN'
import zhCnDataCenter from './modules/dataCenter/zh-CN'
import zhCnFacilityLinkage from './modules/facilityLinkage/zh-CN'
import zhCnTaskPage from './modules/taskPage/zh-CN'
import zhCnLargeScreen from './modules/largeScreen/zh-CN'
import zhCnWarning from './modules/warning/zh-CN'
import zhCnUserRole from './modules/userRole/zh-CN'
import zhCnUserLog from './modules/userLog/zh-CN'
import zhCnMenu from './modules/menu/zh-CN'
import zhCnOverview from './modules/overview/zh-CN'
import zhCnLogin from './modules/login/zh-CN'
import zhCnFacilityList from './modules/facilityList/zh-CN'
import zhCnFacilityLog from './modules/facilityLog/zh-CN'
import zhCnGeneralView from './modules/generalView/zh-CN'
import zhCnNetworking from './modules/networking/zh-CN'
import zhCnFunctional from './modules/functional/zh-CN'
import zhCnDeviceEvents from './modules/deviceEvents/zh-CN'
import zhCnFacilityPattern from './modules/facilityPattern/zh-CN'
import zhCnFacilityGroup from './modules/facilityGroup/zh-CN'
import enUsCommon from './en-US/common'
import enUsDevice from './en-US/device'
import enUsModbus from './en-US/modbus'
import enUsUserOrganize from './en-US/userOrganize'
import enUsUserAccount from './modules/userAccount/en-US'
import enUsDataCenter from './modules/dataCenter/en-US'
import enUsFacilityLinkage from './modules/facilityLinkage/en-US'
import enUsTaskPage from './modules/taskPage/en-US'
import enUsLargeScreen from './modules/largeScreen/en-US'
import enUsWarning from './modules/warning/en-US'
import enUsUserRole from './modules/userRole/en-US'
import enUsUserLog from './modules/userLog/en-US'
import enUsMenu from './modules/menu/en-US'
import enUsOverview from './modules/overview/en-US'
import enUsLogin from './modules/login/en-US'
import enUsFacilityList from './modules/facilityList/en-US'
import enUsFacilityLog from './modules/facilityLog/en-US'
import enUsGeneralView from './modules/generalView/en-US'
import enUsNetworking from './modules/networking/en-US'
import enUsFunctional from './modules/functional/en-US'
import enUsDeviceEvents from './modules/deviceEvents/en-US'
import enUsFacilityPattern from './modules/facilityPattern/en-US'
import enUsFacilityGroup from './modules/facilityGroup/en-US'

const messages = {
  'zh-CN': {
    common: zhCnCommon,
    device: zhCnDevice,
    modbus: zhCnModbus,
    facilityList: zhCnFacilityList,
    facilityLog: zhCnFacilityLog,
    generalView: zhCnGeneralView,
    networking: zhCnNetworking,
    functional: zhCnFunctional,
    deviceEvents: zhCnDeviceEvents,
    userOrganize: zhCnUserOrganize,
    userAccount: zhCnUserAccount,
    dataCenter: zhCnDataCenter,
    facilityLinkage: zhCnFacilityLinkage,
    taskPage: zhCnTaskPage,
    largeScreen: zhCnLargeScreen,
    warning: zhCnWarning,
    userRole: zhCnUserRole,
    userLog: zhCnUserLog,
    menu: zhCnMenu,
    overview: zhCnOverview,
    login: zhCnLogin,
    facilityPattern: zhCnFacilityPattern,
    facilityGroup: zhCnFacilityGroup
  },
  'en-US': {
    common: enUsCommon,
    device: enUsDevice,
    modbus: enUsModbus,
    facilityList: enUsFacilityList,
    facilityLog: enUsFacilityLog,
    generalView: enUsGeneralView,
    networking: enUsNetworking,
    functional: enUsFunctional,
    deviceEvents: enUsDeviceEvents,
    userOrganize: enUsUserOrganize,
    userAccount: enUsUserAccount,
    dataCenter: enUsDataCenter,
    facilityLinkage: enUsFacilityLinkage,
    taskPage: enUsTaskPage,
    largeScreen: enUsLargeScreen,
    warning: enUsWarning,
    userRole: enUsUserRole,
    userLog: enUsUserLog,
    menu: enUsMenu,
    overview: enUsOverview,
    login: enUsLogin,
    facilityPattern: enUsFacilityPattern,
    facilityGroup: enUsFacilityGroup
  }
}

const getDefaultLocale = (): string => {
  const savedLocale = localStorage.getItem('locale')
  if (savedLocale && Object.keys(messages).includes(savedLocale)) {
    return savedLocale
  }
  return 'zh-CN'
}

const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: 'zh-CN',
  messages,
  globalInjection: true,
  missing: (locale, key) => {
    if (process.env.NODE_ENV === 'development') {
      console.warn(`[i18n] Missing translation: ${key} (${locale})`)
      return `⚠️ ${key}`
    }
    return key
  }
})

export default i18n