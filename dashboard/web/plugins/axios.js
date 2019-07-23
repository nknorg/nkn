import {aesEncrypt, aesDecrypt, hmacSHA256} from '~/helpers/crypto'

export default function ({$axios, redirect, store}) {
  $axios.onRequest(config => {
    config.withCredentials = true
    config.headers['Unix'] = Math.floor(Date.now() / 1000)

    if (config.data !== undefined && !(config.data instanceof FormData)) {
      let seed = sessionStorage.getItem('seed')
      let secret = hmacSHA256(seed, store.state.token + store.state.unix)
      config.data = {data: aesEncrypt(JSON.stringify(config.data), secret)}
    }
  })
  $axios.onResponse(resp => {
    if (resp.data.data !== undefined) {
      let padding = 10
      let tick = store.state.unix
      for (let i = tick - padding; i < tick + padding; i++) {
        let seed = sessionStorage.getItem('seed')
        let secret = hmacSHA256(seed, store.state.token + i)
        try {
          let data = aesDecrypt(resp.data.data, secret)
          resp.data = JSON.parse(data)
          return
        } catch (e) {
        }
      }
      throw new Error('data can not decrypt')
    }
  })

  $axios.onError(error => {
    const code = parseInt(error.response && error.response.status)
    // if (code === 500) {
    //   redirect('/error')
    // }
  })
}
