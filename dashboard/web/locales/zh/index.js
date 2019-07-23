import Menu from "./menu"
import Node from "./node"
import Settings from "./settings"
import Neighbor from './neighbor'

export default {
  language: '简体中文',
  menu: Menu,
  node: Node,
  settings: Settings,
  neighbor: Neighbor,

  LOADING: '加载中',
  SEARCH: '搜索',

  NEXT: '下一步',
  CLOSE: '关闭',
  CANCEL: '取消',
  RESTART: '重启',
  START: '开始',
  STOP: '停止',
  SUBMIT: '提交',

  BENEFICIARY: '受益人',
  BALANCE: '余额',
  WALLET_ADDRESS: '钱包地址',
  PUBLIC_KEY: '公钥',
  SECRET_SEED: '私钥种子',
  PASSWORD: '密码',
  PASSWORD_ERROR: '无效的密码.',
  PASSWORD_REQUIRED: '密码不能为空.',
  PASSWORD_HINT: '请输入钱包密码.',

  PASSWORD_CONFIRM: '确认密码',
  PASSWORD_CONFIRM_ERROR: '两次密码不一致.',
  PASSWORD_CONFIRM_REQUIRED: '确认密码不能为空.',
  PASSWORD_CONFIRM_HINT: '请再次输入密码.',

  WALLET_CREATE: '创建钱包',
  WALLET_OPEN: '打开钱包',
  CREATE: '创建',
  OPEN: '打开',
  WALLET_DOWNLOAD: '下载钱包',
  WALLET_UPLOAD: '上传钱包',

  NO_DATA: '没有数据',

  footer: {
    TITLE: 'NKN：新一代互联网的网络基础设施',
    TEXT: 'NKN是区块链技术驱动的一种开放、去中心化的新型网络。NKN倡导用户共享网络资源，鼓励大家构建人人为我，我为人人的共建共享对等网络，在让共建者因协助数据传输而获得经济回报的同时为开发者提供一个开放，便捷，高效和安全的网络平台，让所有人都能体验更好的网络应用和服务。'
  },

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

  }

}
