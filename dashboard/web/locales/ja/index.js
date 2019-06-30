import Menu from "./menu"
import Node from "./node"
import Settings from "./settings"
import Neighbor from './neighbor'

export default {
  language: '日本語',
  menu: Menu,
  node: Node,
  settings: Settings,
  neighbor: Neighbor,

  NEXT: 'ネックスト',
  CLOSE: 'クローズ',
  CANCEL: 'キャンセル',
  RESTART: '再起動',
  START: 'スタト',
  STOP: 'ストップ',
  SUBMIT: '提出する',

  BENEFICIARY: '受益者',
  BALANCE: 'バランス',
  WALLET_ADDRESS: 'ウォレットのアドレス',
  PUBLIC_KEY: '公開鍵',
  PRIVATE_KEY: '秘密鍵',
  PASSWORD: 'パスワード',
  PASSWORD_ERROR: '無効のパスワード.',
  PASSWORD_REQUIRED: 'パスワードは空にできません.',
  PASSWORD_HINT: 'ウォレットパスウードを入力してください.',

  NO_DATA: 'データなし',

  footer:{
    TITLE: 'NKN：新一代互联网的网络基础设施',
    TEXT: 'NKN是区块链技术驱动的一种开放、去中心化的新型网络。NKN倡导用户共享网络资源，鼓励大家构建人人为我，我为人人的共建共享对等网络，在让共建者因协助数据传输而获得经济回报的同时为开发者提供一个开放，便捷，高效和安全的网络平台，让所有人都能体验更好的网络应用和服务。'
  },

  node_status: {
    TITLE: 'ノード状況',
    NODE_STATUS: 'ノード状況',
    NODE_VERSION: 'ノードバージョン',
    RELAY_MESSAGE_COUNT: '节点转发消息数量',
    HEIGHT: 'NKN区块高度',
    BENEFICIARY_ADDR: '受益者のアドレス'
  },
  current_wallet_status: {
    TITLE: 'ランニングウォレット状況',

  }

}
