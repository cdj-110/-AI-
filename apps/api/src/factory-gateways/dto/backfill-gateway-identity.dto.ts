import { IsNotEmpty, IsOptional, IsString, Matches, MaxLength } from 'class-validator';

export class BackfillGatewayIdentityDto {
 @IsString() @Matches(/^[0-9A-Fa-f]{32}$/)
 hardwareId!: string;

 @IsString() @Matches(/^[0-9A-Fa-f]{16}$/)
 chipUsid!: string;

 @IsOptional() @IsString() @IsNotEmpty() @MaxLength(256)
 deviceSecret?: string;
}
