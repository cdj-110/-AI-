import assert from 'node:assert/strict';
import ts from 'typescript';
import {readFile} from 'node:fs/promises';
const source=await readFile(new URL('../src/utils/metric-value.ts',import.meta.url),'utf8');
const javascript=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2020}}).outputText;
const {formatMetricNumber:format,formatMetricValue:display}=await import('data:text/javascript;base64,'+Buffer.from(javascript).toString('base64'));
for(const value of [44.300000000000004,'44.300000000000004']){
 assert.equal(display(value),'44.3');
 assert.equal(display(value,2),'44.30');
}
assert.equal(display('4.4300000000000004e1',2),'44.30');
for(const value of ['001.234','9007199254740993','9007199254740993.0','mbrtu_P01','44.3 dB',''])assert.equal(display(value),value);
assert.equal(display(false),'false');
assert.equal(display(null),'null');
const raw={mbrtu_P01:44.300000000000004};
assert.equal(display(raw.mbrtu_P01,2),'44.30');
assert.equal(raw.mbrtu_P01,44.300000000000004,'display must not alter source values');
for(const view of ['Devices','PlatformDevices','DeviceDetail']){
 const component=await readFile(new URL(`../src/views/${view}.vue`,import.meta.url),'utf8');
 assert.ok(component.includes('formatMetricValue(value'),'all telemetry views must use the shared formatter');
}
assert.equal(format(45.150000000000006,2),'45.15');
assert.equal(format(43.300000000000004),'43.3');
assert.equal(format(0.1+0.2),'0.3');
assert.equal(format(1.234567,6),'1.234567');
assert.equal(format(43.3,2),'43.30');
assert.equal(format(43.3,2,true),'43.3');
assert.equal(format(-0.0001,2),'0.00');
assert.equal(format(12.6,0),'13');
assert.equal(format(0.00000001),'1e-8');
assert.equal(format(123456789012345),'123456789012345');
assert.equal(format(45.150000000000006,99),'45.15');
for(const value of [NaN,Infinity,-Infinity])assert.equal(format(value,2),'-');
console.log('Metric display passed: floating tails, configured decimals 0–6, fallback, tiny values, integers, negative zero and non-finite values.');
