import {div, mul, newNum, NKN_ACC_MUL} from './math'

export function formatFee(fee) {
  if (!fee) return 0
  return div(newNum(fee), NKN_ACC_MUL).toFixed().toString()
}

export function parseFee(fee) {
  if (!fee) return 0
  return mul(newNum(fee), NKN_ACC_MUL).toNumber()
}
