#!/bin/bash

# Make sure the script is run as root user, or someone in the 'docker' group
if [[ ${EUID} -ne 0 ]]; then
  id | grep -q "docker"
  if [[ $? -ne 0 ]]; then
    echo "This script must be run as root, or a user in the 'docker' group"
    exit 1
  fi
fi

usage () {
	echo "Usage: $0 -a <install|remove> -h <host> [-p <port (default 22)>]"
	echo " Uses Docker and SSH to build an ad-hoc server container and deploy it"
	echo " (via the 'install' action) to a previously configured host. The process"
	echo " can be reversed by use of the 'remove' action, which restores the host"
	echo " to its previous configuration."
	echo ""
	echo " This script must be run with root privilges (for access to the docker daemon)"
	echo " You will be prompted for SSH credentials and host-key authentication as needed"
	echo ""
	echo " Options:"
	echo "  -a/--action"
	echo "   Action. Allowed actions are: 'install', 'remove'. Mandatory"
	echo "  -u/--user"
	echo "   Username to connect with. Default 'root'. Optional, but user must belong to the docker group"
	echo "  -h/--host"
	echo "   Host (or IP Address) to connect to. Mandatory"
	echo "  -p/--port"
	echo "   Port to connect to. Default 22. Optional"
	echo ""

  exit 1
}

generate_temporary_credentials () {
	echo "..Generating temporary 4096 bit RSA keypair"
	ssh-keygen -t rsa -b 4096 -C "temp-psiphond-ad_hoc" -f psiphond-ad_hoc -N ""
	PUBLIC_KEY=$(cat psiphond-ad_hoc.pub)

  if [ $USER == "root" ]; then
    echo "${PUBLIC_KEY}" | ssh -o PreferredAuthentications=interactive,password -p $PORT $USER@$HOST "cat >> /$USER/.ssh/authorized_keys"
  else
    echo "${PUBLIC_KEY}" | ssh -o PreferredAuthentications=interactive,password -p $PORT $USER@$HOST "cat >> /home/$USER/.ssh/authorized_keys"
  fi
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

destroy_temporary_credentials () {
	echo "..Removing the temporary key from the remote host"
	PUBLIC_KEY=$(cat psiphond-ad_hoc.pub)

  if [ $USER == "root" ]; then
    ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "sed -i '/temp-psiphond-ad_hoc/d' /$USER/.ssh/authorized_keys"
  else
    ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "sed -i '/temp-psiphond-ad_hoc/d' /home/$USER/.ssh/authorized_keys"
  fi
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi

	echo "..Removing the local temporary keys"
	rm psiphond-ad_hoc psiphond-ad_hoc.pub
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

docker_build_builder () {
	echo "..Building the docker container 'psiphond-build'"
	docker build -f Dockerfile-binary-builder -t psiphond-builder .
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

docker_build_psiphond_binary () {
	echo "..Building 'psiphond' binary"
  cd .. && docker run --rm -v $PWD/.:/go/src/github.com/Psiphon-Labs/psiphon-tunnel-core psiphond-builder
	if [ $? -ne 0 ]; then
		echo "...Failed"
    cd $BASE
		return 1
	fi

  cd $BASE
	stat psiphond > /dev/null 2>&1
	if [ $? -ne 0 ]; then
		echo "...'psiphond' binary file not found"
		return 1
	fi

}

docker_build_psiphond_container () {
	echo "..Building the '${CONTAINER_TAG}' container"
	docker build -t ${CONTAINER_TAG} .
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

save_image () {
	echo "..Saving docker image to archive"
	docker save ${CONTAINER_TAG} | gzip > $ARCHIVE
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi

	stat $ARCHIVE > /dev/null 2>&1
	if [ $? -ne 0 ]; then
		echo "...'${ARCHIVE}' not found"
		return 1
	fi
}

put_and_load_image () {
	echo "..Copying '${ARCHIVE}' to '${HOST}:/tmp'"
	scp -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -P $PORT $ARCHIVE $USER@$HOST:/tmp/
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi

	echo "..Loading image into remote docker"
	ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "zcat /tmp/$ARCHIVE | docker load && rm /tmp/$ARCHIVE"
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

remove_image () {
	echo "..Removing image from remote docker"
  # Single quotes prevents local substitution of variables
  ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST 'docker rmi $(docker images -q psiphond/ad-hoc)'
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

put_systemd_dropin () {
  echo "..Creating ad-hoc environment variables file"
	ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "sed 's/DOCKER_CONTENT_TRUST=1/DOCKER_CONTENT_TRUST=0/' /opt/psiphon/psiphond/config/psiphond.env | sed '/CONTAINER_TAG=/d' > /opt/psiphon/psiphond/config/psiphond.env.adhoc"
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi

	echo "..Creating systemd unit drop-in"
	cat <<- EOF > ad-hoc.conf
	[Service]
	EnvironmentFile=/opt/psiphon/psiphond/config/psiphond.env.adhoc

	# Clear previous pre-start command before setting new one
	# Execute these commands prior to starting the service
	# "-" before the command means errors are not fatal to service startup
	ExecStartPre=
	ExecStartPre=-/usr/bin/docker stop %p-run
	ExecStartPre=-/usr/bin/docker rm %p-run

	# Clear previous start command before setting new one
	ExecStart=
	ExecStart=/usr/bin/docker run --rm \$CONTAINER_PORT_STRING \$CONTAINER_VOLUME_STRING \$CONTAINER_ULIMIT_STRING \$CONTAINER_SYSCTL_STRING --name %p-run ${CONTAINER_TAG}
	EOF

	ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "mkdir -p /etc/systemd/system/psiphond.service.d"
	echo "..Ensuring drop-in directory is available"
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi

	scp -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -P $PORT ad-hoc.conf $USER@$HOST:/etc/systemd/system/psiphond.service.d/
	echo "..Copying drop-in to remote host"
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi

	rm ad-hoc.conf
}

remove_systemd_dropin () {
	echo "..Removing systemd unit drop-in"
	ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "[ ! -f /etc/systemd/system/psiphond.service.d/ad-hoc.conf ] || rm /etc/systemd/system/psiphond.service.d/ad-hoc.conf"
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi

  echo "..Removing ad-hoc environment variables file"
	ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "[ ! -f /opt/psiphon/psiphond/config/psiphond.env.adhoc ] || rm /opt/psiphon/psiphond/config/psiphond.env.adhoc"
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

reload_systemd () {
	echo "..Reloading systemd unit file cache"
	ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "systemctl daemon-reload"
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

restart_psiphond () {
	echo "..Restarting the 'psiphond' service"
	ssh -i psiphond-ad_hoc -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -p $PORT $USER@$HOST "systemctl restart psiphond"
	if [ $? -ne 0 ]; then
		echo "...Failed"
		return 1
	fi
}

install() {
	docker_build_builder
	if [ $? -ne 0 ]; then
		return 1
	fi

	docker_build_psiphond_binary
	if [ $? -ne 0 ]; then
		return 1
	fi

	docker_build_psiphond_container
	if [ $? -ne 0 ]; then
		return 1
	fi

	save_image
	if [ $? -ne 0 ]; then
		return 1
	fi

	put_and_load_image
	if [ $? -ne 0 ]; then
		return 1
	fi

	put_systemd_dropin
	if [ $? -ne 0 ]; then
		return 1
	fi

	reload_systemd
	if [ $? -ne 0 ]; then
		return 1
	fi

	restart_psiphond
	if [ $? -ne 0 ]; then
		return 1
	fi
}

remove () {
	remove_systemd_dropin
	if [ $? -ne 0 ]; then
		echo "...Failed, continuing removal steps. Drop-in file or ad-hoc environment file may still exist"
	fi

	reload_systemd
	if [ $? -ne 0 ]; then
		echo "...Failed, continuing removal steps. 'systemctl daemon-reload' may still be needed to restore previous functionality"
	fi

	restart_psiphond
	if [ $? -ne 0 ]; then
		echo "...Failed, continuing removal steps. The old 'psiphond' may still be running"
	fi

	remove_image
	if [ $? -ne 0 ]; then
		echo "...Final step failed, check for leftover images manually with 'docker images psiphond/ad-hoc', then use 'docker rmi' to remove them"
    destroy_temporary_credentials
		return 1
	fi
}

# Locate and change to the directory containing the script
BASE=$( cd "$(dirname "$0")" ; pwd -P )
cd $BASE

# Validate that we're in a git repository and store the revision
REV=$(git rev-parse --short HEAD)
if [ $? -ne 0 ]; then
	echo "Could not store git revision, aborting"
	exit 1
fi

# Parse command line arguments
while [[ $# -gt 1 ]]; do
	key="$1"

	case $key in
		-a|--action)
			ACTION="$2"
			shift

			if [ "${ACTION}" != "install" ] && [ "${ACTION}" != "remove" ]; then
				echo "Got: '${ACTION}', Expected one of: 'install', or 'remove', aborting."
				exit 1
			fi

			;;
		-u|--user)
			USER="$2"
			shift
			;;
		-h|--host)
			HOST="$2"
			shift
			;;
		-p|--port)
			PORT="$2"
			shift
			;;
		*)
			usage
			;;
	esac
	shift
done

# Validate all mandatory parameters were set
if [ -z $ACTION ]; then
	echo "Action is a required parameter, aborting."
  echo ""
  usage
fi
if [ -z $HOST ]; then
	echo "Host is a required parameter, aborting."
  echo ""
  usage
fi

# Set default values for unset optional paramters
if [ -z $USER ]; then
  USER=root
fi
if [ -z $PORT ]; then
	PORT=22
fi

# Set up other global variables
TIMESTAMP=$(date +'%Y-%m-%d_%H-%M')
CONTAINER_TAG="psiphond/ad-hoc:${TIMESTAMP}"
ARCHIVE="psiphond-ad-hoc.tar.gz"

# Display choices and pause
echo "[$(date)] Ad-Hoc psiphond deploy starting."
echo ""
echo "Configuration:"
echo " - Action: ${ACTION}"
echo " - User: ${USER}"
echo " - Host: ${HOST}"
echo " - Port: ${PORT}"
if [ $ACTION == "install" ]; then
  echo " - Containter Tag: ${CONTAINER_TAG}"
  echo " - Archive Name: ${ARCHIVE}"
fi
echo ""
echo "Pausing 5 seconds to allow for ^C prior to starting"
sleep 5

generate_temporary_credentials
if [ $? -ne 0 ]; then
	echo "Inability to generate temporary credentials is fatal, aborting"
	exit 1
fi

if [ "${ACTION}" == "install" ]; then
  install
  if [ $? -ne 0 ]; then
    echo "....Error during install, rolling back"
    remove
    if [ $? -ne 0 ]; then
      echo "....Rollback failed"
    fi
  fi
elif [ "${ACTION}" == "remove" ]; then
  remove
  if [ $? -ne 0 ]; then
    echo "...Rollback failed"
  fi
else
	echo "Parameter validation passed, but action was not 'install' or 'remove', aborting"
	exit 1
fi

destroy_temporary_credentials
echo "[$(date)] Ad-Hoc psiphond deploy ended."
