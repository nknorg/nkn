<template>
  <v-card min-width="800">
    <v-card-title class="headline">Latest mining rewards</v-card-title>
    <div class="divider"></div>
    <v-card-text>
      <v-layout  wrap>
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
            :items="desserts"
            :search="search"
            prev-icon="chevron_left"
            next-icon="chevron_right"
            sort-icon="arrow_upward"
            :rows-per-page-items="[]"
            :pagination.sync="pagination"
          >
            <template slot="headerCell" slot-scope="{ header }">
              <span class="subheading font-weight-light" v-text="header.text"/>
            </template>
            <template v-slot:items="props">
              <td>{{ props.item.transaction }}</td>
              <td>{{ props.item.value }}</td>
              <td>{{ props.item.height }}</td>
              <td>{{ props.item.time }}</td>
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
  export default {
    name: "LastMiningRewards",
    data: () => ({
      search:'',
      pagination: {
        descending: false,
        rowsPerPage: 10
      },
      headers: [
        {text: 'Transaction', align: 'left', sortable: false, value: 'name'},
        {text: 'Value', align: 'left', sortable: false, value: 'value'},
        {text: 'Height', align: 'left', sortable: false, value: 'height'},
        {text: 'Time', align: 'left', sortable: false, value: 'time'}
      ],
      desserts: [
        {
          transaction: 'Frozen Yogurt',
          value: 159,
          height: 6.0,
          time: 24
        },
        {
          transaction: 'Ice cream sandwich',
          value: 237,
          height: 9.0,
          time: 37
        },
        {
          transaction: 'Eclair',
          value: 262,
          height: 16.0,
          time: 23,


        },
        {
          transaction: 'Cupcake',
          value: 305,
          height: 3.7,
          time: 67,


        },
        {
          transaction: 'Gingerbread',
          value: 356,
          height: 16.0,
          time: 49,


        },
        {
          transaction: 'Jelly bean',
          value: 375,
          height: 0.0,
          time: 94,


        },
        {
          transaction: 'Lollipop',
          value: 392,
          height: 0.2,
          time: 98,


        },
        {
          transaction: 'Honeycomb',
          value: 408,
          height: 3.2,
          time: 87,


        },
        {
          transaction: 'Donut',
          value: 452,
          height: 25.0,
          time: 51,


        },
        {
          transaction: 'KitKat',
          value: 518,
          height: 26.0,
          time: 65,


        }
      ]
    })
  }
</script>

<style scoped>

</style>
