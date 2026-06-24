import { IsObject } from 'class-validator';

export class CreateRemoteConfigDto {
  @IsObject()
  config: Record<string, unknown>;
}
