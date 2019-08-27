<template>
  <v-footer class="pl-0 pr-0" height="auto">
    <v-card flat dark tile class="indigo lighten-1 text-center" width="100%">
      <v-card-text>
        <v-tooltip top v-for="item in icons" :key="item.icon">
          <template v-slot:activator="{ on }">
            <v-btn :href="item.url" target="_blank" class="mx-3 white--text" icon v-on="on">
              <v-icon size="24px">{{item.icon}}</v-icon>
            </v-btn>
          </template>
          <span>{{item.tooltip}}</span>
        </v-tooltip>


      </v-card-text>

      <v-card-text class="white--text pt-0">
        <h4>{{$t('footer.TITLE')}}</h4>
        {{$t('footer.TEXT')}}
      </v-card-text>

      <v-divider></v-divider>

      <v-card-text class="white--text">
        Copyright &copy; 2019 — <strong>nkn-web</strong>
      </v-card-text>
    </v-card>
  </v-footer>
</template>
<script>
  export default {
    data:() =>({
      icons: [
        {icon: 'fab fa-github', tooltip: 'GitHub', url: 'https://github.com/nknorg'},
        {icon: 'fab fa-discord', tooltip: 'Discord', url: 'https://discord.gg/hrZvrVa'},
        {icon: 'fab fa-twitter', tooltip: 'Twitter', url: 'https://twitter.com/NKN_ORG'},
        {icon: 'fas fa-comments', tooltip: 'Forum', url: 'https://forum.nkn.org/'},
        {icon: 'fab fa-youtube', tooltip: 'Youtube', url: 'http://www.youtube.com/c/NKNORG'},
        {icon: 'fab fa-telegram', tooltip: 'Telegram', url: 'https://t.me/nknorg'},
        {icon: 'fab fa-medium', tooltip: 'Medium', url: 'https://medium.com/nknetwork'},
      ]
    })
  }
</script>
