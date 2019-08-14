<template>
    <v-card min-width="800">
        <v-card-title class="headline">{{$t('neighbor.TITLE')}}</v-card-title>
        <div class="divider"></div>
        <v-card-text>
            <v-layout wrap>
                <v-flex xs12 sm6 offset-sm6>
                    <v-text-field
                            v-model="search"
                            append-icon="search"
                            :label="$t('SEARCH')"
                            single-line
                            hide-details
                    ></v-text-field>
                </v-flex>
                <v-flex>
                    <v-data-table
                            :headers="headers"
                            :items="neighbors"
                            :search="search"
                            :no-data-text="$t('NO_DATA')"
                            :footer-props="{
                              itemsPerPageText: this.$t('PER_PAGE_TEXT')
                            }"
                    >
                        <template slot="headerCell" slot-scope="{ header }">
                            <span class="subheading font-weight-light" v-text="header.text"/>
                        </template>
                        <template v-slot:body="{items}">
                            <tbody>
                            <tr v-for="item in items" :key="item.id">
                                <td>{{ item.id }}</td>
                                <td>{{ item.addr }}</td>
                                <td>{{ getStatus(item.syncState) }}</td>
                                <td>{{ getBound(item.isOutbound) }}</td>
                                <td>{{ item.roundTripTime }}</td>
                            </tr>
                            </tbody>

                        </template>
                        <template v-slot:page-text="props">
                            {{ props.pageStart }} - {{ props.pageStop }} of {{ props.itemsLength }}
                        </template>
                    </v-data-table>
                </v-flex>
            </v-layout>
        </v-card-text>
    </v-card>
</template>

<script>
  import {mapState} from 'vuex'

  export default {
    name: "Neighbors",
    computed: {
      ...mapState({
        neighbors: state => state.node.neighbors,
      })
    },
    data() {
      return {
        search: '',
        pagination: {
          descending: false,
          rowsPerPage: 10,
          sortBy: 'id'
        },
        headers: [
          {text: this.$t('neighbor.header.ID'), value: 'id', align: 'left', sortable: false},
          {text: this.$t('neighbor.header.IP'), value: 'addr', align: 'left', sortable: false},
          {text: this.$t('neighbor.header.STATE'), value: 'syncState', align: 'left', sortable: false},
          {text: this.$t('neighbor.header.BOUND'), value: 'isOutbound', align: 'left', sortable: false},
          {text: this.$t('neighbor.header.PING'), value: 'roundTripTime', align: 'left', sortable: false}
        ],
      }
    },
    methods: {
      getStatus(stateStr) {
        const statusEnum = {
          'DEFAULT': this.$t('node.state.DEFAULT'),
          'WAIT_FOR_SYNCING': this.$t('node.state.WAIT_FOR_SYNCING'),
          'SYNC_STARTED': this.$t('node.state.SYNC_STARTED'),
          'SYNC_FINISHED': this.$t('node.state.SYNC_FINISHED'),
          'PERSIST_FINISHED': this.$t('node.state.PERSIST_FINISHED')
        }
        return statusEnum[stateStr] || this.$t('node.state.DEFAULT')
      },
      getBound(str) {
        const statusEnum = {
          true: this.$t('neighbor.OUT_BOUND'),
          false: this.$t('neighbor.IN_BOUND')
        }
        return statusEnum[str]
      },
    }
  }
</script>

<style scoped>

</style>
