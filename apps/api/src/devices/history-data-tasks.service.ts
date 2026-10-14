import { BadRequestException, Injectable, NotFoundException } from '@nestjs/common';
import { IsISO8601, IsString, Length, IsOptional, MaxLength } from 'class-validator';
import { Prisma } from '@prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import { AuthUser } from '../common/decorators/current-user.decorator';
import { deviceAccessWhere } from '../common/user-permissions';
import { DevicesService } from './devices.service';
import { HistoryQueryDto } from './dto/history-query.dto';
import { PickType } from '@nestjs/mapped-types';

export class SaveHistoryTaskDto extends PickType(HistoryQueryDto,['pairs'] as const) {
  @IsString() @Length(1,80) name!: string;
  @IsISO8601() start!: string;
  @IsISO8601() end!: string;
}
export class HistoryTaskFilterDto {
  @IsOptional() @IsString() @MaxLength(100) keyword?:string;
  @IsOptional() @IsString() @MaxLength(36) deviceId?:string;
  @IsOptional() @IsString() @MaxLength(300) pointKey?:string;
  @IsOptional() @IsString() @MaxLength(40) valueMin?:string;
  @IsOptional() @IsString() @MaxLength(40) valueMax?:string;
  @IsOptional() @IsISO8601() start?:string;
  @IsOptional() @IsISO8601() end?:string;
  @IsOptional() @IsString() @MaxLength(10) page?:string;
  @IsOptional() @IsString() @MaxLength(10) pageSize?:string;
}
@Injectable()
export class HistoryDataTasksService {
  constructor(private readonly prisma:PrismaService, private readonly devices:DevicesService) {}
  private scope(actor:AuthUser) {
    if(!actor.tenantId)throw new BadRequestException('请选择租户');
    return {tenantId:actor.tenantId,...(actor.role==='TENANT_USER'?{createdById:actor.sub}:{})};
  }
  async list(actor:AuthUser,page=1,pageSize=20,keyword='') {
    const where={...this.scope(actor),...(keyword.trim()?{name:{contains:keyword.trim().slice(0,80),mode:'insensitive' as const}}:{})};
    const safePage=Math.min(1000000,Math.max(1,Math.floor(Number(page)||1))),size=Math.min(100,Math.max(1,Math.floor(Number(pageSize)||20)));
    const [rows,total,devices]=await Promise.all([this.prisma.historyDataTask.findMany({where,orderBy:{createdAt:'desc'},skip:(safePage-1)*size,take:size}),this.prisma.historyDataTask.count({where}),this.prisma.device.findMany({where:deviceAccessWhere(actor),select:{id:true}})]);
    const allowed=new Set(devices.map(d=>d.id));
    return {items:rows.map(row=>{const pairs=row.pairs as Array<{value1:string}>;const {pairs:_pairs,...summary}=row;return {...summary,pointCount:pairs.length,deviceCount:new Set(pairs.map(p=>p.value1)).size,canView:pairs.every(p=>allowed.has(p.value1))};}),total,page:safePage,pageSize:size};
  }
  private async owned(actor:AuthUser,id:string) {
    const task=await this.prisma.historyDataTask.findFirst({where:{id,...this.scope(actor)}});
    if(!task)throw new NotFoundException('历史数据任务不存在或无权访问');
    return task;
  }
  async get(actor:AuthUser,id:string) {
    const task=await this.owned(actor,id),pairs=task.pairs as Array<{value1:string;value2:string}>;
    const ids=[...new Set(pairs.map(p=>p.value1))];
    const devices=await this.prisma.device.findMany({where:{AND:[deviceAccessWhere(actor),{id:{in:ids}}]},select:{id:true,name:true,deviceKey:true}});
    if(devices.length!==ids.length)throw new NotFoundException('部分设备已删除或权限已变更，暂不能查看此任务');
    return {...task,devices};
  }
  async save(actor:AuthUser,dto:SaveHistoryTaskDto,id?:string) {
    if(id)await this.owned(actor,id);
    const name=dto.name.trim();if(!name)throw new BadRequestException('请填写任务名称');
    if(new Set(dto.pairs.map(p=>`${p.value1}:${p.value2}`)).size!==dto.pairs.length)throw new BadRequestException('不能重复选择同一个点位');
    // Validate the same retention, enabled-point and device scope rules before persisting.
    await this.devices.telemetryHistoryTable(actor,dto.pairs,dto.start,dto.end,1,1);
    const ids=[...new Set(dto.pairs.map(p=>p.value1))];
    const devices=await this.prisma.device.findMany({where:{AND:[deviceAccessWhere(actor),{id:{in:ids}}]},select:{name:true}});
    const data={name,pairs:dto.pairs.map(p=>({value1:p.value1,value2:p.value2})) as Prisma.InputJsonValue,deviceNames:devices.map(d=>d.name),startAt:new Date(dto.start),endAt:new Date(dto.end)};
    return id?this.prisma.historyDataTask.update({where:{id},data}):this.prisma.historyDataTask.create({data:{...data,tenantId:this.scope(actor).tenantId,createdById:actor.sub,createdByName:(actor.username||'用户').slice(0,100)}});
  }
  private conditions(task:{pairs:Prisma.JsonValue;startAt:Date;endAt:Date},filter:HistoryTaskFilterDto) {
    let pairs=task.pairs as Array<{value1:string;value2:string}>;
    if(filter.deviceId)pairs=pairs.filter(p=>p.value1===filter.deviceId);
    if(filter.pointKey)pairs=pairs.filter(p=>JSON.stringify([p.value1,p.value2])===filter.pointKey);
    if(!pairs.length)throw new BadRequestException('筛选设备或点位不属于此任务');
    const number=(v?:string)=>{if(v===undefined||v==='')return undefined;const n=Number(v);if(!v.trim()||!Number.isFinite(n))throw new BadRequestException('数值筛选必须是有效数字');return n;};
    const min=number(filter.valueMin),max=number(filter.valueMax);
    if(min!==undefined&&max!==undefined&&min>max)throw new BadRequestException('数值下限不能大于上限');
    if((min!==undefined||max!==undefined)&&!filter.pointKey)throw new BadRequestException('请先选择具体点位再设置数值范围');
    const start=filter.start?new Date(filter.start):task.startAt,end=filter.end?new Date(filter.end):task.endAt;
    if(!Number.isFinite(start.getTime())||!Number.isFinite(end.getTime())||start<task.startAt||end>task.endAt||start>=end)throw new BadRequestException('筛选时间必须在任务保存的时间范围内');
    return {pairs,start:start.toISOString(),end:end.toISOString(),keyword:filter.keyword,numeric:{min,max}};
  }
  async query(actor:AuthUser,id:string,page?:number,pageSize?:number,filter:HistoryTaskFilterDto={}) {
    const task=await this.get(actor,id);
    const c=this.conditions(task,filter);
    return this.devices.telemetryHistoryTable(actor,c.pairs,c.start,c.end,page??1,pageSize??20,c.keyword,false,c.numeric);
  }
  async export(actor:AuthUser,id:string,filter:HistoryTaskFilterDto={}) {
    const task=await this.get(actor,id);
    const c=this.conditions(task,filter);
    return this.devices.exportHistory(actor,c.pairs,c.start,c.end,c.keyword,c.numeric);
  }
  async remove(actor:AuthUser,id:string){await this.owned(actor,id);await this.prisma.historyDataTask.delete({where:{id}});return {success:true};}
}
