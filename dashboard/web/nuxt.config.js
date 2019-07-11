import colors from 'vuetify/es5/util/colors'

export default {
  router: {
    base: '/web/'
  },
  ssr: false,
  mode: 'spa',
  /*
  ** Headers of the page
  */
  head: {
    titleTemplate: '%s - ' + process.env.npm_package_name,
    title: process.env.npm_package_name || '',
    meta: [
      {name:'theme-color', content:'#3f51b5'},
      {charset: 'utf-8'},
      {name: 'viewport', content: 'width=device-width, initial-scale=1'},
      {hid: 'description', name: 'description', content: process.env.npm_package_description || ''}
    ],
    link: [
      {rel: 'icon', type: 'image/x-icon', href: '/web/favicon.ico'}

    ]
  },
  /*
  ** Customize the progress-bar color
  */
  loading: false,
  /*
  ** Global CSS
  */
  css: [
    '@/styles/global.scss'
  ],
  /*
  ** Plugins to load before mounting the App
  */
  plugins: [
    '~/plugins/axios',
    '~/plugins/i18n',
    '~/plugins/task',
    '~/plugins/router'
  ],
  /*
  ** Nuxt.js modules
  */
  modules: [
    '@nuxtjs/axios',
    '@nuxtjs/vuetify',
    '@nuxtjs/pwa',
    ['nuxt-i18n', {
      // Options
      // vue-i18n configuration
      vueI18n: {},

      // If true, vue-i18n-loader is added to Nuxt's Webpack config
      vueI18nLoader: false,

      // List of locales supported by your app
      // This can either be an array of codes: ['en', 'fr', 'es']
      // Or an array of objects for more complex configurations:
      // [
      //   { code: 'en', iso: 'en-US', file: 'en.js' },
      //   { code: 'fr', iso: 'fr-FR', file: 'fr.js' },
      //   { code: 'es', iso: 'es-ES', file: 'es.js' }
      // ]
      locales: [
        {code: 'en', name: 'English', iso: 'en-US', file: 'en/index.js'},
        {code: 'zh', name: '简体中文', iso: 'zh-CN', file: 'zh/index.js'}
      ],

      // The app's default locale, URLs for this locale won't have a prefix if
      // strategy is prefix_except_default
      defaultLocale: 'en',

      // Separator used to generated routes name for each locale, you shouldn't
      // need to change this
      // routesNameSeparator: '___',

      // Suffix added to generated routes name for default locale if strategy is prefix_and_default,
      // you shouldn't need to change this
      // defaultLocaleRouteNameSuffix: 'default',

      // Routes generation strategy, can be set to one of the following:
      // - 'prefix_except_default': add locale prefix for every locale except default
      // - 'prefix': add locale prefix for every locale
      // - 'prefix_and_default': add locale prefix for every locale and default
      // strategy: 'prefix_except_default',

      // Wether or not the translations should be lazy-loaded, if this is enabled,
      // you MUST configure langDir option, and locales must be an array of objects,
      // each containing a file key
      lazy: true,

      // Directory that contains translations files when lazy-loading messages,
      // this CAN NOT be empty if lazy-loading is enabled
      langDir: 'locales/',

      // Set this to a path to which you want to redirect users accessing root URL (/)
      // rootRedirect: null,

      // Enable browser language detection to automatically redirect user
      // to their preferred language as they visit your app for the first time
      // Set to false to disable
      detectBrowserLanguage: {
        // If enabled, a cookie is set once a user has been redirected to his
        // preferred language to prevent subsequent redirections
        // Set to false to redirect every time
        useCookie: true,
        // Cookie name
        cookieKey: 'i18n_redirected',
        // Set to always redirect to value stored in the cookie, not just once
        alwaysRedirect: false,
        // If no locale for the browsers locale is a match, use this one as a fallback
        fallbackLocale: 'en'
      },

      // Set this to true if you're using different domains for each language
      // If enabled, no prefix is added to your routes and you MUST configure locales
      // as an array of objects, each containing a domain key
      // differentDomains: false,

      // If using different domains, set this to true to get hostname from X-Forwared-Host
      // HTTP header instead of window.location
      // forwardedHost: false,

      // If true, SEO metadata is generated for routes that have i18n enabled
      // Set to false to disable app-wide
      // seo: true,

      // Base URL to use as prefix for alternate URLs in hreflang tags
      // baseUrl: '',

      // By default a store module is registered and kept in sync with the
      // app's i18n current state
      // Set to false to disable
      // vuex: {
      // Module namespace
      // moduleName: 'i18n',

      // Mutations config
      // mutations: {
      // Mutation to commit to store current locale, set to false to disable
      // setLocale: 'I18N_SET_LOCALE',

      // Mutation to commit to store current message, set to false to disable
      // setMessages: 'I18N_SET_MESSAGES'
      // },

      // PreserveState from server
      // preserveState: false
      // },

      // By default, custom routes are extracted from page files using acorn parsing,
      // set this to false to disable this
      // parsePages: true,

      // If parsePages option is disabled, the module will look for custom routes in
      // the pages option, refer to the "Routing" section for usage
      // pages: {
      //   'inspire':{
      //     en:'/locales/en/inspire.js',
      //     zh:'/locales/zh/inspire'
      //   }
      // },

      // By default, custom paths will be encoded using encodeURI method.
      // This does not work with regexp: "/foo/:slug-:id(\\d+)". If you want to use
      // regexp in the path, then set this option to false, and make sure you process
      // path encoding yourself.
      encodePaths: true,

      // Called right before app's locale changes
      // beforeLanguageSwitch: () => null,

      // Called after app's locale has changed
      // onLanguageSwitched: () => null
    }]
  ],
  /*
  ** axios configuration
  */
  axios: {
    withCredentials: true,
    baseURL: '/',
  },
  /*
  ** vuetify module configuration
  ** https://github.com/nuxt-community/vuetify-module
  */
  vuetify: {
    theme: {
      primary: colors.blue.darken2,
      accent: colors.grey.darken3,
      secondary: colors.amber.darken3,
      info: colors.teal.lighten1,
      warning: colors.amber.base,
      error: colors.deepOrange.accent4,
      success: colors.green.accent3
    }
  },
  /*
  ** Build configuration
  */
  build: {

    /*
    ** You can extend webpack config here
    */
    extend(config, ctx) {
      if (ctx.isClient) {
        config.devtool = 'eval-source-map'
      }
    }
  }
}
