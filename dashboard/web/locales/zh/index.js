import Menu from "./menu"
import Node from "./node"

export default {
  language: '简体中文',
  menu: Menu,
  node: Node,

  node_status: {
    TITLE: '节点状态',
    NODE_STATUS: '节点状态',
    NODE_VERSION: '节点版本',
    RELAY_MESSAGE_COUNT: '节点转发消息数量',
    HEIGHT: 'NKN区块高度',
    BENEFICIARY_ADDR: '受益人地址'
  },
  current_wallet_status: {
    TITLE: '运行中的钱包状态',
    BALANCE: '余额',
    WALLET_ADDRESS: '钱包地址',
    PUBLIC_KEY: '公钥',
    PRIVATE_KEY: '私钥'
  }

}
