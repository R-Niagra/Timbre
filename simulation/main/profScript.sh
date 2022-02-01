#!/bin/bash
#This script fethes the heap and go-routine file

echo "Plz enter the IP: "
read IP

echo "Plz enter the port: "
read port
# port=3102

echo "This will ask for the different profiles from address $IP:$port"

# read -p "Enter Directory Name: " dirname

# creates the directory with the name profiles
dirname="profiles"    
if [[ ! -d "$dirname" ]]
then
        if [[ ! -L $dirname ]]
        then
                echo "Directory doesn't exist. Creating now"
                mkdir $dirname
                echo "Directory created"
        else
                echo "Directory exists"
        fi
fi


if [ $port -gt 1000 ]
then 
    echo "Asking for the file"
    END=2
    for ((i=1; ;i++)); do
      echo $i
      
    #   profFile = "heap-${port}-${i}.prof"
    #   touch $file
      #if curl http://$IP:{$port}/debug/pprof/{heap} -o "$dirname/#1-#2-$i.prof"
      if curl http://$IP:{$port}/debug/pprof/{heap} -o "$dirname/#1-#2.prof"
      then
      sleep 0.5
      else 
      echo "Curl failed. Quiting"
      break
      fi

       #if curl http://198.211.114.198:{$port}/debug/pprof/{goroutine} -o "$dirname/#1-#2-$i.prof"
        if curl http://$IP:{$port}/debug/pprof/{goroutine} -o "$dirname/#1-#2.prof"
      then
      sleep 0.5
      else 
      echo "Curl failed. Quiting"
      break
      fi

    done 
else
    echo "Make sure it is a valid port"
fi
