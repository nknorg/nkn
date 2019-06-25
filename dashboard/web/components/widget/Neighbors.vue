<template>
  <v-card min-width="800">
    <v-card-title class="headline">Neighbors</v-card-title>
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
              <td>{{ props.item.id }}</td>
              <td>{{ props.item.ip }}</td>
              <td>{{ props.item.port }}</td>
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
    name: "Neighbors",
    data: () => ({
      search: '',
      pagination: {
        descending: false,
        rowsPerPage: 10
      },
      headers: [
        {text: 'ID', align: 'left', sortable: false, ip: 'id'},
        {text: 'IP', align: 'left', sortable: false, ip: 'ip'},
        {text: 'Port', align: 'left', sortable: false, ip: 'port'}
      ],
      desserts: [
        {
          id: 'Frozen Yogurt',
          ip: '159.169.1.1',
          port: 6.0,
          time: 24
        },
        {
          id: 'Ice cream sandwich',
          ip: '237.169.1.1',
          port: 9.0,
          time: 37
        },
        {
          id: 'Eclair',
          ip: '262.169.1.1',
          port: 16.0,
          time: 23,


        },
        {
          id: 'Cupcake',
          ip: '305.169.1.1',
          port: 3.7,
          time: 67,


        },
        {
          id: 'Gingerbread',
          ip: '356.169.1.1',
          port: 16.0,
          time: 49,


        },
        {
          id: 'Jelly bean',
          ip: '375.169.1.1',
          port: 0.0,
          time: 94,


        },
        {
          id: 'Lollipop',
          ip: '392.169.1.1',
          port: 0.2,
          time: 98,


        },
        {
          id: 'Honeycomb',
          ip: '408.169.1.1',
          port: 3.2,
          time: 87,


        },
        {
          id: 'Donut',
          ip: '452.169.1.1',
          port: 25.0,
          time: 51,


        },
        {
          id: 'KitKat',
          ip: '518.169.1.1',
          port: 26.0,
          time: 65,


        }
      ]
    })
  }
</script>

<style scoped>

</style>
