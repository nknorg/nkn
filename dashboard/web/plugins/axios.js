import {aesEncrypt, aesDecrypt} from '~/helpers/crypto'

export default function ({$axios, redirect, store}) {
  $axios.onRequest(config => {
    config.withCredentials = true
    config.headers['Unix'] = Math.floor(Date.now() / 1000)

    if (config.data !== undefined) {
      let secret = store.state.token + store.state.unix
      config.data = {data: aesEncrypt(JSON.stringify(config.data), secret)}
    }
  })
  $axios.onResponse(resp => {
    if (resp.data.data !== undefined){
      let padding = 10
      let tick = store.state.unix
      for (let i =  tick - padding; i < tick + padding; i++){
        let secret = store.state.token + i
        try{
          let data = aesDecrypt(resp.data.data, secret)
          resp.data = JSON.parse(data)
        }catch (e) {

        }
      }
    }
  })

  $axios.onError(error => {
    const code = parseInt(error.response && error.response.status)
    // if (code === 500) {
    //   redirect('/error')
    // }
  })
}
