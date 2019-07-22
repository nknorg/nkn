import CryptoJS from 'crypto-js'

export function hmacSHA256(message, secret) {
  secret = secret || message
  return CryptoJS.HmacSHA256(message, secret).toString()
}

export function seedHash(password) {
  return sha256(tripleSha256(password))
}

export function authHash(password){
  return sha224(tripleSha256(password))
}

export function passwordHash(password, secret) {
  let passwordHash = sha224(tripleSha256(password))
  return aesEncrypt(passwordHash, secret)
}

export function aesDecrypt(ciphertext, prikey) {
  let key = hmacSHA256(prikey, prikey)
  let iv = CryptoJS.enc.Hex.parse(key.slice(0, 32))
  key = CryptoJS.enc.Hex.parse(key)
  let encryptedHexStr = CryptoJS.enc.Hex.parse(ciphertext)
  let srcs = CryptoJS.enc.Base64.stringify(encryptedHexStr)
  let decrypt = CryptoJS.AES.decrypt(srcs, key, {iv: iv, mode: CryptoJS.mode.CFB, padding: CryptoJS.pad.NoPadding})
  return decrypt.toString(CryptoJS.enc.Utf8)
}

export function aesEncrypt(plaintext, prikey) {
  let srcs = CryptoJS.enc.Utf8.parse(plaintext)
  let key = hmacSHA256(prikey, prikey)

  let iv = CryptoJS.enc.Hex.parse(key.slice(0, 32))
  key = CryptoJS.enc.Hex.parse(key)

  let encrypted = CryptoJS.AES.encrypt(srcs, key, {iv: iv, mode: CryptoJS.mode.CFB, padding: CryptoJS.pad.NoPadding})
  return encrypted.ciphertext.toString()
}

export function cryptoHexStringParse(hexString) {
  return CryptoJS.enc.Hex.parse(hexString)
}

export function sha224(str) {
  return CryptoJS.SHA224(str).toString()
}

export function sha224Hex(hexStr) {
  return sha224(cryptoHexStringParse(hexStr))
}

export function sha256(str) {
  return CryptoJS.SHA256(str).toString()
}

export function sha256Hex(hexStr) {
  return sha256(cryptoHexStringParse(hexStr))
}

export function doubleSha256(str) {
  return CryptoJS.SHA256(CryptoJS.SHA256(str)).toString()
}

export function doubleSha256Hex(hexStr) {
  return CryptoJS.SHA256(CryptoJS.SHA256(cryptoHexStringParse(hexStr))).toString()
}

export function tripleSha256(str) {
  return CryptoJS.SHA256(CryptoJS.SHA256(CryptoJS.SHA256(str))).toString()
}

export function tripleSha256Hex(hexStr) {
  return CryptoJS.SHA256(CryptoJS.SHA256(CryptoJS.SHA256(cryptoHexStringParse(hexStr)))).toString()
}

export function ripemd160(str) {
  return CryptoJS.RIPEMD160(str).toString()
}

export function ripemd160Hex(hexStr) {
  return CryptoJS.RIPEMD160(cryptoHexStringParse(hexStr)).toString()
}
