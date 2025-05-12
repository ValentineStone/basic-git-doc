#!/bin/sh

### BEGIN INIT INFO
# Provides:          basic-git-doc
# Required-Start:    $time $local_fs $remote_fs
# Required-Stop:     $time $local_fs $remote_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: basic-git-doc daemon
# Description:       Init script for the basic-git-doc daemon
### END INIT INFO
#
# Author:       "Leonid Romanovskii" <romanovskii.leonid@gmail.com>
#
set -e

DAEMON=/home/ubuntu/basic-git-doc
PWD=/home/ubuntu
SERVICE_USER=ubuntu
SERVICE_GROUP=ubuntu
SERVICE_RUN_DIR=/home/ubuntu
PIDFILE=$SERVICE_RUN_DIR/basic-git-doc.pid

test -x $DAEMON || exit 0

. /lib/lsb/init-functions

case "$1" in
  start)
        log_daemon_msg "Starting basic-git-doc daemoon" "basic-git-doc"
        if ! test -d $SERVICE_RUN_DIR; then
                mkdir -p $SERVICE_RUN_DIR
                chown -R $SERVICE_USER:$SERVICE_GROUP $SERVICE_RUN_DIR
        fi
        /sbin/start-stop-daemon --start --chdir "$PWD" --exec "$DAEMON" -b --oknodo -c "$SERVICE_USER" -m --pidfile "$PIDFILE"
        #start_daemon -p $PIDFILE $DAEMON
        log_end_msg $?
    ;;
  stop)
        log_daemon_msg "Stopping basic-git-doc daemoon" "basic-git-doc"
        killproc -p $PIDFILE $DAEMON
        log_end_msg $?
    ;;
  status)
        if pidofproc -p $PIDFILE $DAEMON >/dev/null 2>&1; then
            echo "$DAEMON daemoon is running";
            exit 0;
        else
            echo "$DAEMON daemoon is NOT running";
            if test -f $PIDFILE; then exit 2; fi
            exit 3;
        fi
    ;;
  force-reload|restart)
    $0 stop
    $0 start
    ;;
  *)
    echo "Usage: /etc/init.d/basic-git-doc {start|stop|restart|force-reload}"
    exit 1
    ;;
esac

exit 0