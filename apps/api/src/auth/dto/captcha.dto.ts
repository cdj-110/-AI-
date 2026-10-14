import { IsArray, IsInt, IsNotEmpty, IsString, Max, Min, ValidateNested } from 'class-validator';
import { Type } from 'class-transformer';

export class CaptchaTrailPointDto {
  @IsInt()
  @Min(0)
  @Max(320)
  x: number;

  @IsInt()
  @Min(0)
  @Max(15_000)
  t: number;
}

export class VerifyCaptchaDto {
  @IsString()
  @IsNotEmpty()
  challengeId: string;

  @IsInt()
  @Min(0)
  @Max(320)
  offsetX: number;

  @IsArray()
  @ValidateNested({ each: true })
  @Type(() => CaptchaTrailPointDto)
  trail: CaptchaTrailPointDto[];
}
