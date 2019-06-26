export default function ({ $axios, redirect }) {
  $axios.onRequest(config => {
    
  })

  $axios.onError(error => {
    const code = parseInt(error.response && error.response.status)
    if(code === 500) {
      redirect('/error')
    }
  })
}
