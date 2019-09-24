<template>
    <v-app>
        <v-app-bar dark app color="indigo" dense
                   fade-img-on-scroll>

            <v-toolbar-title class="ml-2">
                <h3>
                    <v-avatar tile size="35">
                        <img src="~/static/img/logo.png" alt="avatar">
                    </v-avatar>
                    NKN WEB
                </h3>
            </v-toolbar-title>

            <v-flex class="toolbar" d-flex align-center justify-end>
                <span class="mr-1">{{$t('node_status.NODE_STATUS')}}:</span>
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
                        <v-list-item
                                v-for="locale in availableLocales"
                                :key="locale.code"
                                :to="switchLocalePath(locale.code)"
                                @click="reload"
                        >
                            <v-list-item-content>
                                <v-list-item-title>{{locale.name}}</v-list-item-title>
                            </v-list-item-content>
                        </v-list-item>

                    </v-list>
                </v-menu>
            </v-flex>


            <template v-slot:extension>
                <v-tabs align-with-title background-color="transparent" v-if="update">
                    <v-tab href="#nodeStatus" :to="localePath('nodeStatus')">{{$t('menu.NODE_STATUS')}}</v-tab>
                    <v-tab href="#overview" :to="localePath('overview')">{{$t('menu.OVERVIEW')}}</v-tab>
                    <v-tab href="#settings" :to="localePath('settings')">{{$t('menu.SETTINGS')}}</v-tab>
                    <v-tabs-slider color="pink accent-3"></v-tabs-slider>
                </v-tabs>
            </template>
        </v-app-bar>

        <v-content>
            <v-container fluid grid-list-lg>
                <nuxt/>
            </v-container>
            <Footer/>
        </v-content>

        <WatchOutBoundCount></WatchOutBoundCount>
    </v-app>

</template>

<script>
  import Footer from '~/components/Footer.vue'
  import NodeRunStatus from '~/components/status/NodeRunStatus.vue'
  import WatchOutBoundCount from '~/components/widget/WatchOutBoundCount.vue'
  import {mapState} from 'vuex'
  import '~/styles/status.scss'

  export default {
    components: {
      Footer,
      WatchOutBoundCount,
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
