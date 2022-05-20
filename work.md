Implement a small go-dqlite based software which will:

    Generate a server.crt and server.key on startup
    On bootstrap generates a new database and cluster.crt/cluster.key
    Listen on a provided address/port using a REST API
    Listen on a local unix socket for a local control API
    Support carrying dqlite over that REST API
    Support issuing join tokens over the control API
    Can join an existing deployment by feeding it the join token, will download expected cluster.crt/cluster.key over API
    Keeps track of all participating servers and certificates in the database + in local cache (to allow connection on startup)


This shared bit of code will then be used by both the MicroCeph and MicroCloud snaps to run their own custom little clustered service,
primarily handling tracking of services and roles for their respective workloads.
