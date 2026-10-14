import { OmitType, PartialType } from '@nestjs/mapped-types';
import { CreateProductDto } from './create-product.dto';

// A model code is a stable reference used by licenses/batches; never rename it.
export class UpdateProductDto extends PartialType(OmitType(CreateProductDto, ['code'] as const)) {}
