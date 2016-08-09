#!/bin/bash
set -e

# Set values, that are specific to each project
export VERSION=0.6.1
export PKGNAME=go-stomp-server
export MAINTAINER="Kristina Kovalevskaya <isitiriss@gmail.com>"
export EXENAME="go-stomp-server"
export BUILDPATH="/"

# Building specific values
export CALLER_INFO=true

export DEMON_CONFIG=true
export DEB_USER=stomp

# In which project deploy
export LOGDIR="/var/log/$EXENAME"
export BINDIR="/usr/bin"
export DEMONDIR="/etc/init"
export CONFDIR="/etc/$EXENAME"

# Names of config files
export CONF="$EXENAME.config"
export DEMON_CONF="$EXENAME.conf"
#LOGCONF = "$EXENAME.logconfig"

export PATH_TO_SOURCE="$(pwd)/.."

fakeroot checkinstall -D --pkgversion=$VERSION --pkgname=$PKGNAME \
      --maintainer="\"$MAINTAINER\""  --install=no --fstrans=yes --spec=ABOUT.md --provides="" \
      --pkgsource=$EXENAME ./install.sh

# Deleting unnecessary files
rm -r backup-*tgz

RETVAL=$?
[ $RETVAL -eq 0 ] && echo Success
[ $RETVAL -ne 0 ] && echo Failure
