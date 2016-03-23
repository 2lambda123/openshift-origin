"use strict";

angular.module('openshiftConsole')
  .directive('podStatusChart', function($timeout,
                                        hashSizeFilter,
                                        isTroubledPodFilter,
                                        numContainersReadyFilter,
                                        ChartsService) {
    return {
      restrict: 'E',
      scope: {
        pods: '=',
        desired: '=?'
      },
      templateUrl: 'views/_pod-status-chart.html',
      link: function($scope, element) {
        var chart, config;

        // The phases to show (in order).
        var phases = ["Running", "Not Ready", "Warning", "Failed", "Pending", "Succeeded", "Unknown"];

        $scope.chartId = _.uniqueId('pods-donut-chart-');

        function updateCenterText() {
          var total = hashSizeFilter($scope.pods), smallText;
          if (!angular.isNumber($scope.desired) || $scope.desired === total) {
            smallText = (total === 1) ? "pod" : "pods";
          } else {
            smallText = "scaling to " + $scope.desired + "...";
          }

          ChartsService.updateDonutCenterText(element[0], total, smallText);
        }

        // c3.js config for the pods donut chart
        config = {
          type: "donut",
          bindto: '#' + $scope.chartId,
          donut: {
            expand: false,
            label: {
              show: false
            },
            width: 10
          },
          size: {
            height: 150,
            width: 150
          },
          legend: {
            show: false
          },
          onrendered: updateCenterText,
          tooltip: {
            format: {
              value: function(value, ratio, id) {
                // We add all phases to the data, even if count 0, to force a cut-line at the top of the donut.
                // Don't show tooltips for phases with 0 count.
                if (!value) {
                  return undefined;
                }

                // Disable the tooltip for empty donuts.
                if (id === "Empty") {
                  return undefined;
                }

                // Show the count rather than a percentage.
                return value;
              }
            },
            position: function() {
              // Position in the top-left to avoid problems with tooltip text wrapping.
              return { top: 0, left: 0 };
            }
          },
          data: {
            type: "donut",
            groups: [ phases ],
            // Keep groups in our order.
            order: null,
            colors: {
              // Dummy group for an empty chart. Gray outline added in CSS.
              Empty: "#ffffff",
              Running: "#00b9e4",
              "Not Ready": "#beedf9",
              Warning: "#f9d67a",
              Failed: "#d9534f",
              Pending: "#e8e8e8",
              Succeeded: "#3f9c35",
              Unknown: "#f9d67a"
            },
            selection: {
              enabled: false
            }
          }
        };

        function updateChart(countByPhase) {
          var data = {
            columns: []
          };
          angular.forEach(phases, function(phase) {
            data.columns.push([phase, countByPhase[phase] || 0]);
          });

          if (hashSizeFilter(countByPhase) === 0) {
            // Add a dummy group to draw an arc, which we style in CSS.
            data.columns.push(["Empty", 1]);
          } else {
            // Unload the dummy group if present when there's real data.
            data.unload = "Empty";
          }

          if (!chart) {
            config.data.columns = data.columns;
            chart = c3.generate(config);
          } else {
            chart.load(data);
          }

          // Add to scope for sr-only text.
          $scope.podStatusData = data.columns;
        }

        function countPodPhases() {
          var countByPhase = {};
          var incrementCount = function(phase) {
            countByPhase[phase] = (countByPhase[phase] || 0) + 1;
          };

          var isReady = function(pod) {
            var numReady = numContainersReadyFilter(pod);
            var total = pod.spec.containers.length;

            return numReady === total;
          };

          angular.forEach($scope.pods, function(pod) {
            // Count 'Warning' as its own phase, even if not strictly accurate,
            // so it appears in the donut chart. Warnings are too important not
            // to call out.
            if (isTroubledPodFilter(pod)) {
              incrementCount('Warning');
              return;
            }

            // Also count running, but not ready, as its own phase.
            if (pod.status.phase === 'Running' && !isReady(pod)) {
              incrementCount('Not Ready');
              return;
            }

            incrementCount(pod.status.phase);
          });

          return countByPhase;
        }

        $scope.$watch(countPodPhases, updateChart, true);
        $scope.$watch('desired', updateCenterText);

        $scope.$on('destroy', function() {
          if (chart) {
            // http://c3js.org/reference.html#api-destroy
            chart = chart.destroy();
          }
        });
      }
    };
  });
