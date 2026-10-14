import assert from 'node:assert/strict'
import {defaultPointGroup,firstPopulatedPointGroup} from '../src/pointGroupDefaults.ts'
for(const protocol of ['modbus-rtu','modbus-tcp','modbus-tcp-slave']){
 assert.equal(defaultPointGroup(protocol,[{function:3}]),'3')
 assert.equal(defaultPointGroup(protocol,[{function:3},{function:1}]),'1')
 assert.equal(defaultPointGroup(protocol,[{function:4},{function:2}]),'2')
 assert.equal(defaultPointGroup(protocol,[{}]),'3')
 assert.equal(defaultPointGroup(protocol,[]),'1')
}
for(const protocol of ['iec104','iec104-server']){
 assert.equal(defaultPointGroup(protocol,[{pointType:'M_SP_NA_1'}]),'yx')
 assert.equal(defaultPointGroup(protocol,[{pointType:'C_SC_NA_1'}]),'yk')
 assert.equal(defaultPointGroup(protocol,[{pointType:'M_IT_NA_1'},{pointType:'M_ME_NC_1'}]),'yc')
 assert.equal(defaultPointGroup(protocol,[{pointType:'C_SE_NC_1'},{pointType:'M_DP_NA_1'}]),'yx')
}
for(const protocol of ['siemens-s7','opcua','opcda','iec61850','iec61850-mms-server','iec61850-goose','iec61850-goose-publisher','dlt645'])assert.equal(defaultPointGroup(protocol,[{}]),'all')
assert.equal(firstPopulatedPointGroup(['1','2','3','4'],{'3':20000}),'3')
console.log('Default groups passed: ordered nonempty Modbus/IEC104, all other protocols, empty fallback and complete counts.')
