#!/bin/bash
#
# This library holds all of the logging functions for OpenShift bash scripts.

# os::log::install_cleanup installs os::log::clean_up_logger as a trap on interrupts, termination, and exit.
# If any traps are currently set for these signals, os::log::clean_up_logger is prefixed.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::log::install_cleanup() {
    trap "os::log::clean_up_logger; $(trap -p SIGINT | awk -F"'" '{print $2}')" SIGINT
    trap "os::log::clean_up_logger; $(trap -p SIGTERM | awk -F"'" '{print $2}')" SIGTERM
    trap "os::log::clean_up_logger; $(trap -p EXIT | awk -F"'" '{print $2}')" EXIT
}

# os::log::clean_up_logger should be trapped so that it can stop the logging utility once the script that
# installed it is finished.
# This function stops logging and generates plots of data for easy consumption.
#
# Globals:
#  - LOG_DIR
#  - LOGGER_PID
#  - SAR_LOGFILE
# Arguments:
#  None
# Returns:  
#  None
function os::log::clean_up_logger() {
    local return_code
    return_code=$?

    if jobs -pr | grep -q "${LOGGER_PID}"; then
        kill -SIGTERM "${LOGGER_PID}"
        # give logger ten seconds to gracefully exit before killing it
        for (( i = 0; i < 10; i++ )); do
            if ! jobs -pr | grep -q "${LOGGER_PID}"; then
                # the logger has shutdown, we don't need to wait on it any longer
                break
            fi
        done

        if jobs -pr | grep -q "${LOGGER_PID}"; then
            # the logger has not shutdown, so kill it
            kill -SIGKILL "${LOGGER_PID}"
        fi
    fi

    local log_subset_flags=( "-b" "-B" "-u ALL" "-q" "-r" )

    local log_subset_names=( "iops" "paging" "cpu" "queue" "memory" )

    local log_subset_file
    local i
    for (( i = 0; i < "${#log_subset_flags[@]}"; i++ )); do
        log_subset_file="${LOG_DIR}/${log_subset_names[$i]}.txt"
        # use sadf utility to extract data into easily-parseable format
        sadf -d "${SAR_LOGFILE}" -- ${log_subset_flags[$i]} > "${log_subset_file}"

        local ignored_columns="hostname,interval,"

        # special cases for special output from SAR, because the tool often gives us columns full of baloney
        if [[ "${log_subset_names[$i]}" == "cpu" ]]; then
            ignored_columns="${ignored_columns}CPU,"
        fi

        os::log::internal::prune_datafile "${log_subset_file}" "${ignored_columns}"
        os::log::internal::plot "${log_subset_file}"
    done

    return "${return_code}"
}

# os::log::internal::prune_datafile removes the given columns from a datafile created by 'sadf -d'
#
# Globals:
#  None
# Arguments:
#  - 1: datafile
#  - 2: comma-delimited columns to remove, with trailing comma 
# Returns:
#  None
function os::log::internal::prune_datafile() {
    local datafile=$1
    local column_names=$2

    if [[ "${#column_names}" -eq 0 ]]; then
        return 0
    fi

    local columns_in_order
    columns_in_order=( $( head -n 1 "${datafile}" | sed 's/^# //g' | tr ';' ' ' ) )

    local columns_to_keep
    local i
    for (( i = 0; i < "${#columns_in_order[@]}"; i++ )); do
        if ! echo "${column_names}" | grep -q "${columns_in_order[$i]},"; then
            # this is a column we need to keep, adding one as 'cut' is 1-indexed
            columns_to_keep+=( "$(( $i + 1 ))" )
        fi
    done

    # for the proper flag format for 'cut', we join the list delimiting with commas
    columns_to_keep="$( IFS=','; echo "${columns_to_keep[*]}" )"

    cut --delimiter=';' -f"${columns_to_keep}" "${datafile}" > "${datafile}.tmp"
    sed -i '1s/^/# /' "${datafile}.tmp"
    mv "${datafile}.tmp" "${datafile}"
}

# os::log::internal::plot uses gnuplot to make a plot of some data across time points. This function is intended to be used 
# on the output of a 'sar -f' read of a sar binary file. Plots will be made of all columns and stacked on each other with one x axis.
# This function needs the non-data columns of the file to be prefixed with comments. 
#
# Globals:
#  - LOG_DIR
# Arguments:
#  - 1: data file
# Returns:
#  None
function os::log::internal::plot() {
    local datafile=$1
    local plotname
    plotname="$(basename "${datafile}" .txt)"

    # we are expecting the output of a 'sadf -d' read, so the headers will be on the first line of the file
    local headers
    headers=( $( head -n 1 "${datafile}" | sed 's/^# //g' | tr ';' ' ' ) )

    local records
    local width
    records="$(( $( cat "${datafile}" | wc -l ) - 1 ))" # one of these lines will be the header comment
    if [[ "${records}" -gt 90 ]]; then
        width="$(echo "8.5 + ${records}*0.025" | bc )"
    else 
        width="8.5"
    fi

    local gnuplot_directive=( "set terminal pdf size ${width}in,$(( 2 * (${#headers[@]} - 1) ))in" \
                              "set output \"${LOG_DIR}/${plotname}.pdf\"" \
                              "set datafile separator \";\"" \    
                              "set xdata time" \
                              "set timefmt '%Y-%m-%d %H:%M:%S UTC'" \
                              "set tmargin 1" \
                              "set bmargin 1" \
                              "set lmargin 20" \
                              "set rmargin 20" \
                              "set multiplot layout ${#headers[@]},1 title \"\n${plotname}\n\"" \
                              "unset title" )

    local i
    for (( i = 1; i < "${#headers[@]}"; i++ )); do
        local header
        header="${headers[$i]}"

        if (( $i == ${#headers[@]} - 1 )); then
            # we need x-tick labels on the bottom plot
            gnuplot_directive+=( "set xtics format '%H:%M:%S' rotate by -90" )
        else 
            gnuplot_directive+=( "set format x ''" )
        fi 

        gnuplot_directive+=( "plot \"${datafile}\" using 1:$(( $i + 1 )) title \"${header}\" with lines" )
    done

    # concatenate the array with newlines to get the final directive to send to gnuplot
    gnuplot_directive="$( IFS=$'\n'; echo "${gnuplot_directive[*]}" )"

    echo -e "\$ gnuplot <<< ${gnuplot_directive}\n" >> "${LOG_DIR}/gnuplot.log"
    gnuplot <<< "${gnuplot_directive}" >> "${LOG_DIR}/gnuplot.log" 2>&1
    echo -e "\n\n" >> "${LOG_DIR}/gnuplot.log"

    echo "[INFO] Stacked plot for log subset \"${plotname}\" written to ${LOG_DIR}/${plotname}.pdf"
}


# os::log::install_system_logger installs the system logger and begins logging
#
# Globals:
#  - LOG_DIR
# Arguments:
#  None
# Returns:
#  - export LOGGER_PID
#  - export SAR_LOGFILE
function os::log::install_system_logger() {
    readonly SAR_LOGFILE="${LOG_DIR}/sar.log"
    export SAR_LOGFILE

    ( os::log::internal::run_system_logger "${SAR_LOGFILE}" "${LOG_DIR}/sar_stderr.log" ) &
    LOGGER_PID=$!
    readonly LOGGER_PID
    export LOGGER_PID
}


# os::log::internal::run_system_logger runs the system logger. This function should be run in the background.
# 'sar' is configured to run once a second for 24 hours, so the cleanup trap should be installed to ensure that
# the process is killed once the parent script is finished.
#
# Globals:
#  None
# Arguments:
#  - 1: file to log binary outut to
#  - 2: file to log stderr of the logger to
# Returns:
#  None
function os::log::internal::run_system_logger() {
    local binary_logfile=$1
    local stderr_logfile=$2

    sar -A -o "${binary_logfile}" 1 86400 1>/dev/null 2>"${stderr_logfile}"
}