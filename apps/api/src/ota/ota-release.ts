import { BadRequestException } from '@nestjs/common';

export function assertForwardOtaRelease(installedRelease: bigint | null | undefined, packageRelease: bigint) {
  if (installedRelease !== null && installedRelease !== undefined && packageRelease <= installedRelease) {
    throw new BadRequestException(`设备当前发布序号为 ${installedRelease.toString()}，请选择大于该序号的升级包`);
  }
}
