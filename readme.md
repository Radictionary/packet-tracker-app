# My wireshark clone
dockerfile and docker-compose coming soon to self-host

Note: Change app.InProduction to true when in production

- Frontend:
  - Uses plain CSS and Javascript

- Backend:
  - Built in Go version 1.20
  - Uses the [chi router](github.com/go-chi/chi)
  - Uses [alex edwards scs session management](github.com/alexedwards/scs)
  - Contains embedded database for persisting logic


### To-Do
- [ ] More filter options including support for more specific protocols
- [ ] More visual customization options
- [ ] Better persisting data and optimizations
- [ ] Packet Dump more organized when shown
