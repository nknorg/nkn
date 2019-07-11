<template>
  <v-app>
    <v-toolbar :clipped-left="clipped" dark app tabs color="indigo" dense>
      <v-toolbar-title class="ml-2">
        <v-avatar tile size="30">
          <img src="~/static/img/logo.png" alt="avatar">
        </v-avatar>
        NKN WEB
      </v-toolbar-title>
      <v-spacer></v-spacer>
      <NodeRunStatus :node-status="this.nodeStatus"></NodeRunStatus>
      <div class="divider-vertical mx-3"></div>
      <v-badge color="transparent">
        <span>{{$t('node_status.NODE_VERSION')}}: {{nodeStatus.version}}</span>
      </v-badge>

      <div class="divider-vertical mx-3"></div>
      <v-menu offset-y>
        <template v-slot:activator="{ on }">
          <v-btn icon v-on="on">
            <v-icon>language</v-icon>
          </v-btn>
        </template>
        <v-list>
          <v-list-tile
            v-for="locale in availableLocales"
            :key="locale.code"
            :to="switchLocalePath(locale.code)"
            @click="reload"
          >
            <v-list-tile-title>{{ locale.name }}</v-list-tile-title>
          </v-list-tile>

        </v-list>
      </v-menu>
    </v-toolbar>
    <v-content>
      <v-container fluid grid-list-lg>
        <nuxt/>
      </v-container>
      <Footer/>
    </v-content>
  </v-app>
</template>

<script>
  import Footer from '~/components/Footer.vue'
  import NodeRunStatus from '~/components/status/NodeRunStatus.vue'
  import {mapState} from 'vuex'
  import '~/styles/status.scss'

  export default {
    components: {
      Footer,
      NodeRunStatus
    },
    data() {
      return {
        clipped: false,
        update: true
      }
    },
    computed: {
      ...mapState({
        nodeStatus: state => state.node.nodeStatus,
      }),
      availableLocales() {
        return this.$i18n.locales
      }
    },
    methods: {
      reload() {
        setTimeout(() => { // prevent execution before link to
          this.update = false
          this.$nextTick(() => {
            this.update = true
          })
        }, 10)
      }
    }
  }
</script>
