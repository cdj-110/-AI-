export interface GeographyType {
    geographyValue:string,
    loading:boolean,
    optionConfig:Array<{id:string,name:string}>
}

export interface boxGroupingListType {
    group_id:string,
    group_name:string
}

export interface argeConfigType {
    disabled?:boolean,
    from?:{
        useMode:string,
        macAddr:string,
        ipAddr:string,
        interpretation:string,
        getway:string,
        preferDns:string,
        spareDns:string,
        routeText:string,
        senior:""
    }
}