import {ServiceStatusEnum} from '~/helpers/consts'

export default ({app, redirect, store}) => {
  app.router.beforeEach((to, from, next) => {
    if (to.path === '/loading') {
      return next()
    }

    next()
  })

  app.router.afterEach((to, from) => {
    let status = store.state.serviceStatus
    if (status === ServiceStatusEnum.SERVICE_STATUS_DEFAULT) {
      redirect(app.localePath('loading'))
    } else if ((status & ServiceStatusEnum.SERVICE_STATUS_NO_WALLET_FILE) > 0) {
      if (!~to.path.indexOf('/wallet/create')) {
        redirect(app.localePath('wallet-create'))
      }
    } else if ((status & ServiceStatusEnum.SERVICE_STATUS_NO_PASSWORD) > 0) {
      if (!~to.path.indexOf('/wallet/open')) {
        redirect(app.localePath('wallet-open'))
      }
    }else{
      let seed = sessionStorage.getItem('seed')
      if (!seed) {
        redirect(app.localePath('wallet-open'))
      }
    }
  })
}
