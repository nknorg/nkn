import Menu from './menu'
import Node from './node'

export default {
  language: 'English',
  menu: Menu,
  node: Node,

  NEXT: 'Next',
  CLOSE: 'Close',
  CANCEL: 'Cancel',

  BALANCE: 'Balance',
  WALLET_ADDRESS: 'Wallet address',
  PUBLIC_KEY: 'Public key',
  PRIVATE_KEY: 'Private key',
  PASSWORD: 'Password',
  PASSWORD_REQUIRED: 'Password is required.',
  PASSWORD_HINT: 'Please enter wallet password.',

  footer:{
    TITLE: 'NKN: Network Infra for Decentralized Internet',
    TEXT: 'NKN is the new kind of P2P network connectivity protocol & ecosystem powered by a novel public blockchain. We use economic incentives to motivate Internet users to share network connection and utilize unused bandwidth. NKN\'s open, efficient, and robust networking infrastructure enables application developers to build the decentralized Internet so everyone can enjoy secure, low cost, and universally accessible connectivity.'
  },

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

  }
}
