#ifndef INITIALRESPONSE_H
#define INITIALRESPONSE_H

#include <QUdpSocket>
#include <QtEndian>
#include "dnsinfo.h"

/* YourFriendlyDNS - A really awesome multi-platform (lin,win,mac,android) local caching and proxying dns server!
Copyright (C) 2018  softwareengineer1 @ github.com/softwareengineer1
Support my work by sending me some Bitcoin or Bitcoin Cash in the value of what you valued one or more of my software projects,
so I can keep bringing you great free and open software and continue to do so for a long time!
I'm going entirely 100% free software this year in 2018 (and onwards I want to) :)
Everything I make will be released under a free software license! That's my promise!
If you want to contact me another way besides through github, insert your message into the blockchain with a BCH/BTC UTXO! ^_^
Thank you for your support!
BCH: bitcoincash:qzh3knl0xeyrzrxm5paenewsmkm8r4t76glzxmzpqs
BTC: 1279WngWQUTV56UcTvzVAnNdR3Z7qb6R8j
(These are the payment methods I currently accept,
if you want to support me via another cryptocurrency let me know and I'll probably start accepting that one too)

This program is free software; you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation; either version 2 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License along
with this program; if not, write to the Free Software Foundation, Inc.,
51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA. */



void morphRequestIntoARecordResponse(QByteArray &dnsrequest, quint32 responseIP, quint32 spliceOffset, quint32 ttl = 13337);
void morphRequestIntoARecordResponse(QByteArray &dnsrequest, std::vector<quint32> &responseIPs, quint32 spliceOffset, quint32 ttl = 13337);

class InitialResponse : public QObject
{
public:
    Q_OBJECT
public:
    explicit InitialResponse(DNSInfo &dns, QObject *parent=0);

private:
    //QUdpSocket sock;
    QDateTime timeWithoutAResponse;
    DNSInfo respondTo;
    bool responseHandled;

signals:
    void finished();

public slots:
    void lookupDoneSendResponseNow(DNSInfo &dns, QUdpSocket *serversocket);
    void deleteObjectsTheresNoResponseFor();
};

#endif // INITIALRESPONSE_H
