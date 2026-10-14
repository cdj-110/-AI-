import { Type } from 'class-transformer';
import { ArrayMaxSize, ArrayNotEmpty, IsArray, IsISO8601, IsInt, IsOptional, IsString, IsUUID, Length, Matches, Max, Min, ValidateNested } from 'class-validator';

class HistoryPointDto {
  @IsUUID() value1!: string;
  @IsString() @Length(1,100) @Matches(/^[\w.\-:]+$/) value2!: string;
}
export class HistoryQueryDto {
  @IsArray() @ArrayNotEmpty() @ArrayMaxSize(20) @ValidateNested({each:true}) @Type(()=>HistoryPointDto) pairs!: HistoryPointDto[];
  @IsOptional() @IsISO8601() start?: string;
  @IsOptional() @IsISO8601() end?: string;
  @IsOptional() @Type(()=>Number) @IsInt() @Min(1) @Max(1000000) page?: number;
  @IsOptional() @Type(()=>Number) @IsInt() @Min(1) @Max(100) pageSize?: number;
  @IsOptional() @IsString() @Length(0,100) keyword?: string;
}
