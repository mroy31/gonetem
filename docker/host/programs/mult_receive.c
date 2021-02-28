
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netdb.h>
#include <ctype.h>
#include <stdio.h>
#include <errno.h>
#include <stdlib.h>
#include <unistd.h>
#include <signal.h>


/* Not everyone has the headers for this, so improvise */
#ifndef MCAST_JOIN_SOURCE_GROUP
#define MCAST_JOIN_SOURCE_GROUP 46

struct group_source_req
{
    /* Interface index.  */
    uint32_t gsr_interface;

    /* Group address.  */
    struct sockaddr_storage gsr_group;

    /* Source address.  */
    struct sockaddr_storage gsr_source;
};
#endif


const int BUFFSIZE = 512;
int udp_sock = -1;


/*
 * SIGTERM Handler
 */
void close_handler(int sig) {
    close(udp_sock);
    exit(0);
}


/*
 * Main function
 */
int main(int argc, char **argv) {
    int c, receive;
    int port = 52220; int idx = 1;
    char buffer[BUFFSIZE];

    int is_ssm = 0;
    struct group_source_req group_source_req;
    struct ip_mreq multi_addr;
    struct sockaddr_in *group;
    struct sockaddr_in *source;

    struct sockaddr_in source_addr, rec_addr;
    int addr_length = sizeof(source_addr);

    /*
     * params
     *   -s @source : use SSM address instead of SM address
     */
    while ((c=getopt(argc,argv,"s:")) != -1) {
        switch(c) {
            case 's': // activate ssm reception
                is_ssm = 1;

                source_addr.sin_family=AF_INET;
                inet_pton(AF_INET, optarg, &(source_addr.sin_addr));

                source=(struct sockaddr_in*)&group_source_req.gsr_source;
                source->sin_family = AF_INET;
                inet_aton(optarg, &source->sin_addr);
                source->sin_port = 0;
	        break;
        }
    }

    // create udp socket
    if ((udp_sock=socket(AF_INET,SOCK_DGRAM,0))==-1) {
        perror("Unable to create udp socket");
        exit(1);
    }

    // receive udp multicast paquet from specified source and address
    if (!is_ssm) { // IGMPv2 report
        /* Group is 239.1.1.1 */
        multi_addr.imr_interface.s_addr = htonl(INADDR_ANY);
        inet_pton(AF_INET, "239.1.1.1", &(multi_addr.imr_multiaddr));
        setsockopt(udp_sock, IPPROTO_IP, IP_ADD_MEMBERSHIP,
                                 &multi_addr, sizeof(multi_addr));
    } else { // IGMPv3 report
        group_source_req.gsr_interface = 0;
        group=(struct sockaddr_in*)&group_source_req.gsr_group;

        /* Group is 232.1.1.1 */
        group->sin_family = AF_INET;
        inet_pton(AF_INET, "232.1.1.1", &group->sin_addr);
        group->sin_port = 0;

        setsockopt(udp_sock, IPPROTO_IP, MCAST_JOIN_SOURCE_GROUP, &group_source_req,
                    sizeof(group_source_req));
    }

    // prepare receiver address and bind udp socket
    rec_addr.sin_family=AF_INET;
    rec_addr.sin_port=htons(port);
    rec_addr.sin_addr.s_addr = htonl(INADDR_ANY);
    if (bind(udp_sock, (struct sockaddr *) &rec_addr, addr_length) == -1) {
        perror("erreur dans le bind udp");
        close(udp_sock);
        exit(1);
    }

    // catch signals
    signal (SIGTERM, close_handler);
    signal (SIGKILL, close_handler);
    signal (SIGINT, close_handler);

    while(1) {
        if((receive = recvfrom(udp_sock, (char *) buffer, BUFFSIZE,
                      0, (struct sockaddr *)&source_addr, &addr_length)) < 0) {
            perror("Unable to receive multicast paquet");
            close(udp_sock);
            exit(1);
        }
        printf("Receive %d paquet\n", idx);
        idx++;
    }

    // close the socket
    if (close(udp_sock) == -1) {
        perror("Unable to close udp socket");
        exit(1);
    }

    return(0);
}

