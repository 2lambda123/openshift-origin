'use strict';

angular.module('openshiftConsole')
  .directive('logViewer', [
    'DataService',
    'logLinks',
    function(DataService, logLinks) {

      // Create a template for each log line that we clone below.
      var logLineTemplate = $('<div row class="log-line"/>');
      $('<div class="log-line-number"><div row flex main-axis="end"></div></div>').appendTo(logLineTemplate);
      $('<div flex class="log-line-text"/>').appendTo(logLineTemplate);

      // Keep a reference the DOM node rather than the jQuery object for cloneNode.
      logLineTemplate = logLineTemplate.get(0);

      return {
        restrict: 'AE',
        transclude: true,
        templateUrl: 'views/directives/logs/_log-viewer.html',
        scope: {
          kind: '@',
          name: '=',
          context: '=',
          options: '=?',
          status: '=?',
          start: '=?',
          end: '=?',
          chromeless: '=?'
        },
        controller: [
          '$scope',
          function($scope) {
            $scope.loading = true;

            // Default to false. Let the user click the follow link to start auto-scrolling.
            $scope.autoScroll = false;

            // Set to true before auto-scrolling.
            var autoScrollingNow = false;
            var onScroll = function() {
              // Determine if the user scrolled or we auto-scrolled.
              if (autoScrollingNow) {
                // Reset the value.
                autoScrollingNow = false;
              } else {
                // If the user scrolled the window manually, stop auto-scrolling.
                $scope.$evalAsync(function() {
                  $scope.autoScroll = false;
                });
              }
            };
            $(window).scroll(onScroll);

            var scrollBottom = function() {
              // Tell the scroll listener this is an auto-scroll. The listener
              // will reset it to false.
              autoScrollingNow = true;
              logLinks.scrollBottom();
            };

            var toggleAutoScroll = function() {
              $scope.autoScroll = !$scope.autoScroll;
              if ($scope.autoScroll) {
                // Scroll immediately. Don't wait the next message.
                scrollBottom();
              }
            };

            var scrollTop = function() {
              // Stop auto-scrolling when the user clicks the scroll top link.
              $scope.autoScroll = false;
              logLinks.scrollTop();
            };

            var buffer = document.createDocumentFragment();

            // https://lodash.com/docs#debounce
            var update = _.debounce(function() {
              // Display all buffered lines.
              var logContent = document.getElementById('logContent');
              logContent.appendChild(buffer);

              // Clear the buffer.
              buffer = document.createDocumentFragment();

              // Follow the bottom of the log if auto-scroll is on.
              if ($scope.autoScroll) {
                scrollBottom();
              }
            }, 100, { maxWait: 300 });

            // maintaining one streamer reference & ensuring its closed before we open a new,
            // since the user can (potentially) swap between multiple containers
            var streamer;
            var stopStreaming = function(keepContent) {
              if (streamer) {
                streamer.stop();
                streamer = null;
              }

              if (!keepContent) {
                // Cancel any pending updates. (No-op if none pending.)
                update.cancel();
                $('#logContent').empty();
                buffer = document.createDocumentFragment();
              }
            };

            var streamLogs = function() {
              // Stop any active streamer.
              stopStreaming();

              if (!$scope.name) {
                return;
              }

              angular.extend($scope, {
                loading: true,
                error: false,
                autoScroll: false,
                limitReached: false
              });

              var options = angular.extend({
                follow: true,
                tailLines: 1000,
                limitBytes: 10 * 1024 * 1024 // Limit log size to 10 MiB
              }, $scope.options);
              streamer =
                DataService.createStream($scope.kind, $scope.name, $scope.context, options);

              var lastLineNumber = 0;

              var addLine = function(lineNumber, text) {
                lastLineNumber++;

                // Append the line to the document fragment buffer.
                var line = logLineTemplate.cloneNode(true);
                line.childNodes[0].childNodes[0].appendChild(document.createTextNode(lineNumber));
                line.lastChild.appendChild(document.createTextNode(text));
                buffer.appendChild(line);

                update();
              };

              streamer.onMessage(function(msg, raw, cumulativeBytes) {
                if (options.limitBytes && cumulativeBytes >= options.limitBytes) {
                  $scope.$evalAsync(function() {
                    $scope.limitReached = true;
                    $scope.loading = false;
                  });
                  stopStreaming(true);
                }

                addLine(lastLineNumber, msg);

                // Show the start and end links once we have at least 1 line
                if (!$scope.showScrollLinks && lastLineNumber > 1) {
                  $scope.$evalAsync(function() {
                    $scope.showScrollLinks = true;
                  });
                }

                // Warn the user if we might be showing a partial log.
                if (!$scope.largeLog && lastLineNumber >= options.tailLines) {
                  $scope.$evalAsync(function() {
                    $scope.largeLog = true;
                  });
                }
              });

              streamer.onClose(function() {
                streamer = null;
                $scope.$evalAsync(function() {
                  angular.extend($scope, {
                    loading: false,
                    autoScroll: false
                  });
                });
              });

              streamer.onError(function() {
                streamer = null;
                $scope.$evalAsync(function() {
                  angular.extend($scope, {
                    loading: false,
                    error: true,
                    autoScroll: false
                  });
                });
              });

              streamer.start();
            };

            $scope.$watchGroup(['name', 'options.container'], streamLogs);

            $scope.$on('$destroy', function() {
              // Close streamer if open. (No-op if not streaming.)
              stopStreaming();

              // Stop listening for scroll events.
              $(window).off('scroll', onScroll);
            });

            angular.extend($scope, {
              ready: true,
              scrollBottom: logLinks.scrollBottom,
              scrollTop: scrollTop,
              toggleAutoScroll: toggleAutoScroll,
              goChromeless: logLinks.chromelessLink,
              restartLogs: streamLogs
            });
          }
        ]
      };
    }
  ]);
