import { Transform } from 'class-transformer';
import { ArrayMaxSize, ArrayMinSize, ArrayUnique, IsArray, IsIn, IsNotEmpty, IsString, IsUUID, MaxLength } from 'class-validator';

export class SaveDeviceGroupDto {
  @Transform(({ value }) => typeof value === 'string' ? value.trim() : value)
  @IsString()
  @IsNotEmpty()
  @MaxLength(50)
  name: string;
}

export class AssignDeviceGroupDto {
  @IsArray()
  @ArrayMinSize(1)
  @ArrayMaxSize(100)
  @ArrayUnique()
  @IsUUID('all', { each: true })
  deviceIds: string[];

  @IsArray()
  @ArrayMaxSize(100)
  @ArrayUnique()
  @IsUUID('all', { each: true })
  groupIds: string[];

  @IsIn(['ADD', 'REMOVE', 'REPLACE'])
  mode: 'ADD' | 'REMOVE' | 'REPLACE';
}
