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
            label="Search"
            single-line
            hide-details
          ></v-text-field>
        </v-flex>
        <v-flex>
          <v-data-table
            :headers="headers"
            :items="neighbors"
            :search="search"
            prev-icon="chevron_left"
            next-icon="chevron_right"
            sort-icon="arrow_upward"
            :no-data-text="$t('NO_DATA')"
            :pagination.sync="pagination"
            rows-per-page-text=""
            :rows-per-page-items="[0]"
          >
            <template slot="headerCell" slot-scope="{ header }">
              <span class="subheading font-weight-light" v-text="header.text"/>
            </template>
            <template v-slot:items="props">
              <td>{{ props.item.id }}</td>
              <td>{{ props.item.addr }}</td>
              <td>{{ getStatus(props.item.syncState) }}</td>
              <td>{{ props.item.isOutbound }}</td>
              <td>{{ props.item.roundTripTime }}</td>
            </template>
            <template v-slot:pageText="props">
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
    data: () => ({
      search: '',
      pagination: {
        descending: false,
        rowsPerPage: 10,
        sortBy: 'id'
      },
      headers: [
        {text: 'ID', value: 'id', align: 'left', sortable: false},
        {text: 'Address', value: 'addr', align: 'left', sortable: false},
        {text: 'Sync state', value: 'syncState', align: 'left', sortable: false},
        {text: 'Is outbound', value: 'isOutbound', align: 'left', sortable: false},
        {text: 'Round Trip Time', value: 'roundTripTime', align: 'left', sortable: false}
      ],

    }),
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
    }
  }
</script>

<style scoped>

</style>