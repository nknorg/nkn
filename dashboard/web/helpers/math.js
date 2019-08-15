import {Decimal} from 'decimal.js'

export const NKN_ACC_MUL = newNum(100000000)
export const NKN_DP_LENGTH = 8

export function newNum(arg) {
  return new Decimal(arg)
}


export function add(arg1, arg2) {
  return Decimal.add(arg1, arg2)
}

/***
 *
 * @param arg1
 * @param arg2
 * @returns {Decimal}
 */
export function sub(arg1, arg2) {
  return Decimal.sub(arg1, arg2)
}

/***
 *
 * @param arg1
 * @param arg2
 * @returns {Decimal}
 */
export function mul(arg1, arg2) {
  return Decimal.mul(arg1, arg2)
}

export function mulAndFloor(arg1, arg2) {
  return Decimal.mul(arg1, arg2).floor()
}

/***
 *
 * @param arg1
 * @param arg2
 * @returns {Decimal}
 */
export function div(arg1, arg2) {
  return Decimal.div(arg1, arg2)
}

/***
 *
 * @param arg1
 * @param arg2
 * @returns {boolean}
 */
export function eq(arg1, arg2) {
  return new Decimal(arg1).eq(arg2)
}

/**
 *
 * @param base
 * @param target
 * @returns {boolean}
 */
export function greaterThan(base, target) {
  return new Decimal(base).greaterThan(target)
}

/**
 *
 * @param base
 * @param target
 * @returns {boolean}
 */
export function greaterThanOrEqualTo(base, target) {
  return new Decimal(base).greaterThanOrEqualTo(target)
}

/**
 *
 * @param base
 * @param target
 * @returns {boolean}
 */
export function lessThan(base, target) {
  return new Decimal(base).lessThan(target)
}

/***
 *
 * @param base
 * @param target
 * @returns {boolean}
 */
export function lessThanOrEqualTo(base, target) {
  return new Decimal(base).lessThanOrEqualTo(target)
}

/***
 *
 * @param arg
 * @returns {string}
 */
export function int2HexString(arg) {
  let hexString = new Decimal(arg).floor().toHexadecimal().toString().substring(2)

  if (1 === hexString.length % 2) {
    hexString = '0' + hexString
  }

  return hexString
}

/***
 *
 * @param arg
 * @returns {*}
 */
export function hexString2Int(arg) {
  return ('0x' === arg.slice(0, 2)) ? new Decimal(arg) : new Decimal("0x" + arg)
}
