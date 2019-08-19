import Menu from './menu'
import Node from './node'
import Settings from './settings'
import Neighbor from './neighbor'
import Config from './config'
import Tooltip from './tooltip'

export default {
  language: 'English',
  menu: Menu,
  node: Node,
  settings: Settings,
  neighbor: Neighbor,
  config: Config,
  tooltip: Tooltip,

  LOADING: 'loading',
  SEARCH: 'Search',

  NEXT: 'Next',
  CLOSE: 'Close',
  CANCEL: 'Cancel',
  RESTART: 'Restart',
  START: 'Start',
  STOP: 'Stop',
  SUBMIT: 'Submit',
  RESET: 'Reset',

  BENEFICIARY: 'Beneficiary',
  BALANCE: 'Balance',
  WALLET_ADDRESS: 'Wallet address',
  PUBLIC_KEY: 'Public key',
  SECRET_SEED: 'Secret seed',
  PASSWORD: 'Password',
  PASSWORD_ERROR: 'Invalid password.',
  PASSWORD_REQUIRED: 'Password is required.',
  PASSWORD_HINT: 'Please enter wallet password.',
  FIELD_REQUIRED: 'This field is required.',
  NUMBER_FIELD_ERROR: 'This field must be a number.',
  POSITIVE_NUMBER_FIELD_ERROR: 'This field cannot be negative.',
  NUMBER_MIN_ERROR: 'This field must be greater than {min}',
  NUMBER_MAX_ERROR: 'This field must be less than {max}',
  NUMBER_DP_ERROR: 'The number of decimal places cannot exceed {dp} digits',

  PASSWORD_CONFIRM: 'Please enter password again',
  PASSWORD_CONFIRM_ERROR: 'Password does not match.',
  PASSWORD_CONFIRM_REQUIRED: 'Please enter password again.',
  PASSWORD_CONFIRM_HINT: 'Please enter password confirm.',

  WALLET_CREATE: 'Create wallet',
  WALLET_OPEN: 'Open wallet',
  CREATE: 'Create',
  OPEN: 'Open',
  WALLET_DOWNLOAD: 'Download wallet',
  WALLET_UPLOAD: 'Upload wallet',

  NO_DATA: 'No data available',
  PER_PAGE_TEXT: 'Rows per pages',

  footer: {
    TITLE: 'NKN: Network Infra for Decentralized Internet',
    TEXT: 'NKN is the new kind of P2P network connectivity protocol & ecosystem powered by a novel public blockchain. We use economic incentives to motivate Internet users to share network connection and utilize unused bandwidth. NKN\'s open, efficient, and robust networking infrastructure enables application developers to build the decentralized Internet so everyone can enjoy secure, low cost, and universally accessible connectivity.'
  },

  node_status: {
    TITLE: 'Node status',
    NODE_STATUS: 'Node status',
    NODE_VERSION: 'Node version',
    ID: 'ID',
    RELAY_MESSAGE_COUNT: 'Message relayed by node',
    HEIGHT: 'NKN network block height',
    BENEFICIARY_ADDR: 'Beneficiary address'
  },
  current_wallet_status: {
    TITLE: 'Current wallet status',

  }
}
