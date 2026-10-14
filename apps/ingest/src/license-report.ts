import { Prisma } from '@prisma/client';

interface FactoryIdentity {
 sn: string; hardwareId: string; chipUsid?: string|null; licenseSequence: number;
 issuedLicense?: unknown;
}

// Called only for broker-authenticated factory topics. Never trust a payload's
// claimed identity to choose the database row, and never persist arbitrary JSON.
export function parseLicenseReport(payload: Buffer, gateway: FactoryIdentity): Prisma.InputJsonObject|undefined {
 if(payload.length>16384)return;
 let report: any;
 try{report=JSON.parse(payload.toString('utf8')).authorization;}catch{return;}
 if(!report||typeof report!=='object'||Array.isArray(report))return;
 if(report.sn!==gateway.sn||report.hardwareId!==gateway.hardwareId||
   (gateway.chipUsid&&report.chipUsid!==gateway.chipUsid))return;
 if(!/^[0-9A-F]{32}$/.test(report.hardwareId)||typeof report.chipUsid!=='string'||!/^[0-9A-F]{16}$/.test(report.chipUsid))return;
 if(report.licenseVersion!==2||!Number.isSafeInteger(report.licenseSequence)||report.licenseSequence<1||report.licenseSequence>gateway.licenseSequence)return;
 if(typeof report.keyId!=='string'||!/^[A-Za-z0-9._-]{1,64}$/.test(report.keyId)||typeof report.licenseId!=='string'||!/^[a-f0-9]{64}$/.test(report.licenseId))return;
 if(typeof report.valid!=='boolean'||typeof report.restricted!=='boolean'||(report.valid&&report.restricted))return;
 if(!Array.isArray(report.features)||report.features.length>64||report.features.some((value: unknown)=>typeof value!=='string'||!/^[A-Za-z0-9._-]{1,64}$/.test(value as string)))return;
 if(typeof report.issuedAt!=='string'||typeof report.expiresAt!=='string')return;
 const issued=new Date(report.issuedAt),expires=new Date(report.expiresAt);
 if(!Number.isFinite(issued.getTime()))return;
 if(report.expiresAt!==''&&(!Number.isFinite(expires.getTime())||issued>=expires))return;
 const expiresAt=report.expiresAt===''?'':expires.toISOString();
 const expected=gateway.issuedLicense as Record<string,any>|null;
 if(report.licenseSequence===gateway.licenseSequence&&expected){
  if(report.licenseId!==expected.licenseId||report.keyId!==expected.keyId||
    issued.toISOString()!==expected.issuedAt||expiresAt!==expected.expiresAt||
    JSON.stringify(report.features)!==JSON.stringify(expected.features))return;
 }
 return {sn:report.sn,hardwareId:report.hardwareId,chipUsid:report.chipUsid,
  licenseVersion:report.licenseVersion,licenseSequence:report.licenseSequence,
  keyId:report.keyId,licenseId:report.licenseId,issuedAt:issued.toISOString(),expiresAt,
  features:report.features,valid:report.valid,restricted:report.restricted};
}

export async function persistLicenseReport(tx: Pick<Prisma.TransactionClient,'factoryGateway'>,
 gateway: FactoryIdentity & {id:string;status:string}, report: Prisma.InputJsonObject, now:Date){
 const where={id:gateway.id,licenseSequence:gateway.licenseSequence,status:gateway.status};
 const accepted=await tx.factoryGateway.updateMany({where:{...where,
  AND:[{OR:[{licenseReportedAt:null},{licenseReportedAt:{lt:now}}]},
   {OR:[{licenseReportedAt:null},{reportedLicense:{path:['licenseSequence'],lte:Number(report.licenseSequence)}}]}]},
  data:{reportedLicense:report,licenseReportedAt:now}});
 if(accepted.count&&report.valid&&!report.restricted&&report.licenseSequence===gateway.licenseSequence&&(report.expiresAt===''||new Date(String(report.expiresAt))>now)){
  await tx.factoryGateway.updateMany({where:{...where,licenseActivatedAt:null},data:{licenseActivatedAt:now}});
 }
 return accepted.count>0;
}
