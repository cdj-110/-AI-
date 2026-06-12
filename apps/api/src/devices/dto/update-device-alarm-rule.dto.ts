import { PartialType } from '@nestjs/mapped-types';
import { CreateDeviceAlarmRuleDto } from './create-device-alarm-rule.dto';

export class UpdateDeviceAlarmRuleDto extends PartialType(CreateDeviceAlarmRuleDto) {}
