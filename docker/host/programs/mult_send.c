
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <netinet/in.h>
#include <netdb.h>
#include <ctype.h>
#include <stdio.h>
#include <errno.h>
#include <stdlib.h>
#include <unistd.h>
#include <signal.h>

const int TTL = 20;
const int BUFFSIZE = 512;
int udp_sock = -1;
int run = 1;


/*
 * SIGTERM Handler
 */
void close_handler(int sig) {
    run = 0;
}


/*
 * Main function
 */
int main(int argc, char **argv) {
    int c;
    int ip_addr = htonl(0xef010101); // 239.1.1.1
    int port = 52220;

    char buffer[BUFFSIZE];
    struct sockaddr_in remote_addr;
    int addr_length = sizeof(remote_addr);

    /*
     * params
     *   -s : use SSM address instead of SM address
     */
    while ((c=getopt(argc,argv,"s")) != -1) {
        switch(c) {
            case 's':
                ip_addr = htonl(0xe8010101); // 232.1.1.1
	        break;
        }
    }

    // prepare remote address struct
    remote_addr.sin_addr.s_addr = ip_addr;
    remote_addr.sin_family=AF_INET;
    remote_addr.sin_port=htons(port);

    // create udp socket
    if ((udp_sock=socket(AF_INET,SOCK_DGRAM,0))==-1) {
        perror("Unable to create udp socket");
        exit(1);
    }

    // catch signals
    signal (SIGINT, close_handler);
    signal (SIGTERM, close_handler);
    signal (SIGKILL, close_handler);

    // send udp multicast paquet
    setsockopt(udp_sock, IPPROTO_IP, IP_MULTICAST_TTL, &TTL, sizeof(TTL));
    while(run) {
        sendto(udp_sock,buffer,BUFFSIZE, 0,
                (const struct sockaddr*) &remote_addr, addr_length);
        sleep(1);
    }

    // close the socket
    if (close(udp_sock) == -1) {
        perror("Unable to close udp socket");
        exit(1);
    }

    return(0);
}

