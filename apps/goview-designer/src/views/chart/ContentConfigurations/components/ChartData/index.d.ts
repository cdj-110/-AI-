import { RequestHttpEnum, RequestHttpIntervalEnum, RequestDataTypeEnum, SelectHttpTimeNameObj } from '@/enums/httpEnum'

// 匹配结果
export enum DataResultEnum {
    NULL = 0,
    SUCCESS = 1,
    FAILURE = 2,
}

export enum TimelineTitleEnum {
    FILTER = '数据过滤',
    MAPPING = '数据映射',
    CONTENT = '数据内容',
}

export enum SelectCreateDataEnum {
    STATIC = '静态数据',
    AJAX = '动态请求',
    Pond = '公共接口',
}

export interface SelectCreateDataType {
    label: SelectCreateDataEnum
    value: RequestDataTypeEnum
    disabled?: boolean
}

// ajax 请求类型
export interface SelectHttpType {
    label: RequestHttpEnum
    value: RequestHttpEnum
    disabled?: boolean
    style?: object
}

// 类型选项
export const selectTypeOptions: SelectHttpType[] = [
    {
        label: RequestHttpEnum.shishi,
        value: RequestHttpEnum.shishi,
        // style: {
        //   color: 'greenyellow',
        //   fontWeight: 'bold'
        // }
    },
    {
        label: RequestHttpEnum.lishi,
        value: RequestHttpEnum.lishi,
        style: {}
    }
]

// ajax 请求间隔
export interface SelectHttpTimeType {
    label: string
    value: RequestHttpIntervalEnum
    disabled?: boolean
}

// 时间选项
export const selectTimeOptions: SelectHttpTimeType[] = []