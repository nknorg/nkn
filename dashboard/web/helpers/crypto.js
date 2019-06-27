import CryptoJS from 'crypto-js'

export function cryptoHexStringParse(hexString) {
  return CryptoJS.enc.Hex.parse(hexString)
}

export function sha256(str) {
  return CryptoJS.SHA256(str).toString();
}

export function sha256Hex(hexStr) {
  return sha256(cryptoHexStringParse(hexStr));
}

export function doubleSha256(str) {
  return CryptoJS.SHA256(CryptoJS.SHA256(str)).toString();
}

export function doubleSha256Hex(hexStr) {
  return CryptoJS.SHA256(CryptoJS.SHA256(cryptoHexStringParse(hexStr))).toString();
}

export function ripemd160(str) {
  return CryptoJS.RIPEMD160(str).toString();
}

export function ripemd160Hex(hexStr) {
  return CryptoJS.RIPEMD160(cryptoHexStringParse(hexStr)).toString();
}
