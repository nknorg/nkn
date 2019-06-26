import Menu from './menu'
import Node from './node'

export default {
  language: 'English',
  menu: Menu,
  node: Node,

  node_status: {
    TITLE: 'Node status',
    NODE_STATUS: 'Node status',
    NODE_VERSION: 'Node version',
    RELAY_MESSAGE_COUNT: 'Message relayed by node',
    HEIGHT: 'NKN network block height',
    BENEFICIARY_ADDR: 'Beneficiary address'
  },
  current_wallet_status: {
    TITLE: 'Current wallet status',
    BALANCE: 'Balance',
    WALLET_ADDRESS: 'Wallet address',
    PUBLIC_KEY: 'Public key',
    PRIVATE_KEY: 'Private key'
  }
}
