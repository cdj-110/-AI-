import assert from 'node:assert/strict'
import { POINT_TABLE_RESIZE_HIT_RADIUS, pointTableResizeColumn } from '../src/tableResizeHitArea.ts'

assert.equal(POINT_TABLE_RESIZE_HIT_RADIUS,10)

// The divider at x=100 can be grabbed from both adjacent columns.
assert.equal(pointTableResizeColumn(90,0,100,0),0)
assert.equal(pointTableResizeColumn(100,0,100,0),0)
assert.equal(pointTableResizeColumn(110,100,200,1),0)
assert.equal(pointTableResizeColumn(111,100,200,1),-1)

// Away from a divider, header clicks remain column-selection clicks.
assert.equal(pointTableResizeColumn(50,0,100,0),-1)
assert.equal(pointTableResizeColumn(150,100,200,1),-1)
assert.equal(pointTableResizeColumn(5,0,100,0),-1)

// Every divider still belongs to the column on its left.
assert.equal(pointTableResizeColumn(190,100,200,1),1)
assert.equal(pointTableResizeColumn(210,200,300,2),1)

console.log('Point table resize hit area: 20px around each divider; selection area preserved')
