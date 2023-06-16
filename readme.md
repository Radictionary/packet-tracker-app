# My wireshark clone
dockerfile and docker-compose coming soon to self-host in a single binary

Note: Change app.InProduction to true when in production

- Frontend:
  - Uses plain CSS and Javascript

- Backend:
  - Built in Go version 1.20
  - Uses the [chi router](github.com/go-chi/chi)
  - Uses Redis Database


### To-Do
- [ ] More filter options including support for more specific protocols
- [x] More visual customization options
- [x] Save current packets in a pcap file
- [x] Packet Dump more organized when shown
- [ ] Be able to choose location of pcap save and choose file format of saving packet capture
- [x] Open pcap files
- [x] Better Data Persistance with addition of database
- [x] DNS packet optimization and display of DNS query and more information

