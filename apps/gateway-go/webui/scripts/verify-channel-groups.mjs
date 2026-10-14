import assert from 'node:assert/strict'
import {groupChannels,groupResourceTree} from '../src/channelGroups.ts'
const channels=[{channelKey:'f1',role:'forward'},{channelKey:'c1',role:'collect'},{channelKey:'f2',role:'forward',enabled:false},{channelKey:'legacy'}]
const input=JSON.stringify(channels)
assert.deepEqual(groupChannels(channels).map(group=>group.channels.map(channel=>channel.channelKey)),[['c1','legacy'],['f1','f2']])
const resources=[{resourceKey:'ethernet',channels},{resourceKey:'empty',channels:[]},{resourceKey:'rs485',channels:[{channelKey:'serial',role:'collect'}]}]
const groups=groupResourceTree(resources)
assert.deepEqual(groups.map(resource=>resource.resourceKey),['ethernet','empty','rs485'])
assert.deepEqual(groups[0].groups.map(group=>group.channels.map(channel=>channel.channelKey)),[['c1','legacy'],['f1','f2']])
assert.notEqual(groups[0].groups[0].treeKey,groups[0].groups[1].treeKey,'collection and forwarding must fold independently')
assert.equal(groups[0].groups[0].channels[0],channels[1],'channel identity and device references must be preserved')
assert.deepEqual(groups[1].groups.map(group=>group.channels),[[],[]],'empty resources retain both creation entries')
assert.equal(JSON.stringify(channels),input,'grouping must not mutate or sort saved configuration')
assert.deepEqual(groupResourceTree([]),[])
console.log('Channel grouping: roles, stable order, legacy defaults, empty resources, independent keys and no config mutation passed')
