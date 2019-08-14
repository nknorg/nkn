<template>
  <v-app>
    <v-app-bar dark app color="indigo" dense
               fade-img-on-scroll>

      <v-toolbar-title class="ml-2">
        <h3 class="pt-0 mt-2">
          <v-avatar tile size="35">
            <img src="~/static/img/logo.png" alt="avatar">
          </v-avatar>
          NKN WEB
        </h3>
      </v-toolbar-title>

      <v-flex class="toolbar" d-flex align-center justify-end>

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
              <v-list-tile-title>{{ locale.name }}</v-list-tile-title>
            </v-list-item>

          </v-list>
        </v-menu>
      </v-flex>


    </v-app-bar>
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
