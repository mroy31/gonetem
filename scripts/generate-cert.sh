#!/bin/sh

DURATION=3650
rm *.pem *.srl

# 1. Generate CA's private key and self-signed certificate
openssl req -x509 -newkey rsa:4096 -days ${DURATION} -nodes -keyout ca-key.pem -out ca-cert.pem \
    -subj "/C=FR/ST=Occitanie/L=Toulouse/O=Tech School/OU=Education/CN=*.rca.enac.fr/emailAddress=rca@enac.fr"

echo "CA's self-signed certificate"
openssl x509 -in ca-cert.pem -noout -text

# 2. Generate web server's private key and certificate signing request (CSR)
openssl req -newkey rsa:4096 -nodes -keyout server-key.pem -out server-req.pem \
    -subj "/C=FR/ST=Occitanie/L=Toulouse/O=Tech School/OU=Education/CN=*gonetem-server.rca.enac.fr/emailAddress=rca@enac.fr"

# 3. Use CA's private key to sign web server's CSR and get back the signed certificate
openssl x509 -req -in server-req.pem -days ${DURATION} -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out server-cert.pem -extfile server-ext.cnf

echo "Server's signed certificate"
openssl x509 -in server-cert.pem -noout -text

# 4. Generate client's private key and certificate signing request (CSR)
openssl req -newkey rsa:4096 -nodes -keyout client-key.pem -out client-req.pem \
    -subj "/C=FR/ST=Occitanie/L=Toulouse/O=Tech School/OU=Education/CN=*gonetem-client.rca.enac.fr/emailAddress=rca@enac.fr"

# 5. Use CA's private key to sign client's CSR and get back the signed certificate
openssl x509 -req -in client-req.pem -days ${DURATION} -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out client-cert.pem -extfile client-ext.cnf

echo "Client's signed certificate"
openssl x509 -in client-cert.pem -noout -text

rm ca-key.pem ca-cert.srl