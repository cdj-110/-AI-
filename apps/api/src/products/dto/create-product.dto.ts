import { IsBoolean, IsInt, IsNotEmpty, IsOptional, IsString, Max, MaxLength, Min } from 'class-validator';

export class CreateProductDto {
  @IsString() @IsNotEmpty() @MaxLength(32)
  code: string;

  @IsString() @IsNotEmpty() @MaxLength(64)
  name: string;

  @IsOptional() @IsInt() @Min(0) @Max(64)
  ethernetPortCount?: number | null;

  @IsOptional() @IsInt() @Min(0) @Max(64)
  // Compatibility only: old records may have a total but no interface types.
  serialPortCount?: number | null;

  @IsOptional() @IsInt() @Min(0) @Max(64)
  rs232PortCount?: number | null;

  @IsOptional() @IsInt() @Min(0) @Max(64)
  rs485PortCount?: number | null;

  @IsOptional() @IsInt() @Min(0) @Max(64)
  canPortCount?: number | null;

  @IsOptional() @IsBoolean()
  supports4G?: boolean | null;

  @IsOptional() @IsBoolean()
  supportsWifi?: boolean | null;

  @IsOptional() @IsBoolean()
  enabled?: boolean;

  @IsOptional() @IsString() @MaxLength(512)
  description?: string | null;
}
