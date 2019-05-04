#/bin/bash

function libcheck(){

      # Check for curl library files.

      SP_LIBCFILE3=$(find / -name "libcurl.so.3" 2> /dev/null)
      SP_LIBCFILE4=$(find / -name "libcurl.so.4" 2> /dev/null)
      SP_LIBCFILE3=$(echo $SP_LIBCFILE3 | cut -d ' ' -f1)
      SP_LIBCFILE4=$(echo $SP_LIBCFILE4 | cut -d ' ' -f1)
      SP_LIBCBASE=$(find / -name "libcurl.so" 2> /dev/null)
      SP_LIBCBASE=$(echo $SP_LIBCBASE | cut -d ' ' -f1)
      SP_LIBCPATH=$(find / -name libcurl.so -printf '%h\n' 2> /dev/null)

    # If default library doesn't exist search for native ones.
      if [[ $SP_LIBCPATH == "" ]];then
        # Preform the check for both of them.
        if [[ $SP_LIBCFILE3 == "" ]];then
          SP_LIBCBASE=$(find / -name "libcurl.so.4" 2> /dev/null)
          SP_LIBCBASE=$(echo $SP_LIBCBASE | cut -d ' ' -f1)
          SP_LIBCPATH=$(find / -name libcurl.so.4 -printf '%h\n' 2> /dev/null)
        else
          SP_LIBCBASE=$(find / -name "libcurl.so.3" 2> /dev/null)
          SP_LIBCBASE=$(echo $SP_LIBCBASE | cut -d ' ' -f1)
          SP_LIBCPATH=$(find / -name libcurl.so.3 -printf '%h\n' 2> /dev/null)
        fi
      fi
  # Continue with the required configuration.
      SP_LIBCPATH=$(echo $SP_LIBCPATH | cut -d ' ' -f1)
      SP_LIBC3="libcurl.so.3"
      SP_LIBC4="libcurl.so.4"
      if [[ $SP_LIBCFILE3 == "" ]];then
              if [ -e $SP_LIBCFILE3 ];then
                ln -s $SP_LIBCBASE $SP_LIBCPATH/$SP_LIBC3
              fi
      fi
      if [[ $SP_LIBCFILE4 == "" ]];then
              if [ -L $SP_LIBCFILE4 ];then
                ln -s $SP_LIBCBASE $SP_LIBCPATH/$SP_LIBC4
              fi
      fi
}

  SP_ARCH=$(uname -m)
  if [ $SP_ARCH != 'x86_64' ];then
    	download_src=https://www.saaspass.com/desktop/unix/x32/unixssh.tar.gz #DownloadLink 32Bit
              __download_dst=$2
              __download_src=$1
    	# Check for libcurl.so.3 or libcurl.so.4
      libcheck
  else
      download_src=https://www.saaspass.com/desktop/unix/x64/unixssh.tar.gz #DownloadLink 64bit

            __download_src=$1
            __download_dst=$2
        #Lib check
        libcheck
  fi

function download {
  # $1: The source URL.
  # $2: The local file to write to.
  # Following block is to determen if 32bit or 64bit is used

	       __download_src=$1
         __download_dst=$2


  if which curl >/dev/null; then
    (set -x; curl -# -f "$__download_src" > "$__download_dst")
  elif which wget >/dev/null; then
    (set -x; wget -O - "$__download_src" > "$__download_dst"
)  else
    echo "Either curl or wget must be installed to download files.";
    return 1
  fi
}

# Ask the user a yes-or-no question, and get put "1" or nothing in the target
# variable, so it can be tested in an if statement.
function promptYesNo {
  # $1: The question being asked.
  # $2: The default answer, 't' or not 'y'.
  # $3: The name of the variable to put "1" or nothing in.
  if [ -n "$CORE_DISABLE_PROMPTS" ]; then
    eval $3="$2"
    return
  fi
  __pyn_default="y/N"
  if [ "$2" == "y" ]; then
    __pyn_default="Y/n"
  fi
  __pyn_message="$1 ($__pyn_default): "
  read -p "$__pyn_message" __pyn_response
  if [ -z "$__pyn_response" ]; then
    __pyn_response=$2
  fi
  if [ "$__pyn_response" == "y" -o "$__pyn_response" == "Y" ]; then
    eval $3=1
  else
    eval $3=
  fi
}

# Prompt the user for some string input, with a default. If the user enters
# nothing, use the default. Put the result in the target variable.
function promptWithDefault {
  # $1: The question being asked.
  # $2: The default answer.
  # $3: The name of the variable to put the result in.
  if [ -n "$CORE_DISABLE_PROMPTS" ]; then
    __pwd_response=$2
  else
    __pwd_message="$1 ($2): "
    read -p "$__pwd_message" __pwd_response
    if [ -z "$__pwd_response" ]; then
      __pwd_response=$2
    fi
  fi
  # Expand embedded $... and leading ~... but do not glob.
  set -o noglob
  __pwd_variable=$3
  eval set -- $__pwd_response
  eval $__pwd_variable="'$*'"
  set +o noglob
}

function install {
  PREFIX="/opt"
	default_install=$PREFIX
	download_dst=$scratch/unixssh.tar.gz

	download "$download_src" "$download_dst" || return
  	echo

  	__install_msg="Directory to extract under (this will create a directory unixssh"
  	promptWithDefault "$__install_msg" "$default_install" install_dst
  	mkdir -p "$install_dst"

  	destination="$install_dst/unixssh"

  # Make sure the destination is clear.
  destination="$install_dst/unixssh"
  while [ -e "$destination" ]; do
    echo "\"$destination\" already exists."
    promptYesNo "Remove it and unpack?" n REMOVE_OLD
    if [ -n "$REMOVE_OLD" ]; then
      (set -x; rm -rf "$destination")
    else
      # Give the opportunity to put it somewhere else, or try again.
      promptWithDefault "$__install_msg" "$default_install" install_dst
      mkdir -p "$install_dst"
      destination="$install_dst/unixssh"
    fi
  done

  # Extract the bundle to the destination.
  (set -x; tar -C "$install_dst" -xvf "$download_dst") || return
  echo

  # Run the bundled install script.
  echo  SPDIR="$install_dst/unixssh" > /tmp/SP_INSTALL
  (set -x; "$destination/install.sh") || return  	

}

PS4=
scratch=$(mktemp -d -t tmp.XXXXXXXXXX) && trap 'rm -rf "$scratch"' EXIT || exit 1
install < /dev/tty


