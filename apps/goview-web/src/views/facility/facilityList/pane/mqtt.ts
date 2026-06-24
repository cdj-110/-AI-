import mqtt from 'mqtt'

class SimpleMqtt {
  constructor() {
    this.client = null
    this.isConnected = false
  }

  // 连接
  connect(host: string, clientId = 'vue_' + Date.now()) {
    this.client = mqtt.connect(host, { clientId })
    
    this.client.on('connect', () => {
      console.log('MQTT连接成功')
      this.isConnected = true
    })

    this.client.on('message', (topic, message) => {
      console.log(`收到消息 [${topic}]: ${message.toString()}`)
    })

    this.client.on('error', (error) => {
      console.error('MQTT错误:', error)
    })
  }

  // 订阅
  subscribe(topic) {
    if (!this.isConnected) return false
    this.client.subscribe(topic)
    return true
  }

  // 发布
  publish(topic, message) {
    if (!this.isConnected) return false
    this.client.publish(topic, message)
    return true
  }

  // 断开连接
  disconnect() {
    if (this.client) {
      this.client.end()
      this.client = null
      this.isConnected = false
    }
  }
}

export default new SimpleMqtt()