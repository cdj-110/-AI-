const assert=require('node:assert/strict');
const {retryUntilReady,isAbortError}=require('../dist/startup-readiness');

(async()=>{
  let attempts=0;const failures=[];
  await retryUntilReady(async()=>{attempts++;if(attempts<3)throw new Error('dependency unavailable')},{retryMs:1,onFailure:(error,attempt)=>failures.push([error.message,attempt])});
  assert.equal(attempts,3);assert.deepEqual(failures,[['dependency unavailable',1],['dependency unavailable',2]]);

  const controller=new AbortController();let abortedAttempts=0;
  const stopped=retryUntilReady(async()=>{abortedAttempts++;throw new Error('still unavailable')},{retryMs:5000,signal:controller.signal});
  await new Promise(resolve=>setTimeout(resolve,5));controller.abort();
  await assert.rejects(stopped,error=>isAbortError(error));assert.equal(abortedAttempts,1);

  let keepRunning=true,guardedAttempts=0;
  const guarded=retryUntilReady(async()=>{guardedAttempts++;keepRunning=false;throw new Error('connection changed')},{retryMs:5000,shouldRetry:()=>keepRunning});
  await assert.rejects(guarded,error=>isAbortError(error));assert.equal(guardedAttempts,1);
  console.log('Ingest startup readiness passed: transient dependency failures retry until recovery, and shutdown aborts pending retry without an extra attempt.');
})().catch(error=>{console.error(error);process.exitCode=1});
