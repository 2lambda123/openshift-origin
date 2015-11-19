'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:PodController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('PodController', function ($scope, $routeParams, $timeout, DataService, project, $filter, ImageStreamResolver, MetricsService) {
    $scope.pod = null;
    $scope.imageStreams = {};
    $scope.imagesByDockerReference = {};
    $scope.imageStreamImageRefByDockerReference = {}; // lets us determine if a particular container's docker image reference belongs to an imageStream
    $scope.builds = {};
    $scope.alerts = {};
    $scope.renderOptions = $scope.renderOptions || {};
    $scope.renderOptions.hideFilterWidget = true;
    $scope.logOptions = {};
    $scope.terminalTabWasSelected = false;
    $scope.breadcrumbs = [
      {
        title: "Pods",
        link: "project/" + $routeParams.project + "/browse/pods"
      },
      {
        title: $routeParams.pod
      }
    ];

    // Check for a ?tab=<name> query param to allow linking directly to a tab.
    if ($routeParams.tab) {
      $scope.selectedTab = {};
      $scope.selectedTab[$routeParams.tab] = true;
    }

    var watches = [];

    // Check if the metrics service is available so we know when to show the tab.
    MetricsService.isAvailable().then(function(available) {
      $scope.metricsAvailable = available;
    });

    project.get($routeParams.project).then(function(resp) {
      var context = {
        project: resp[0],
        projectPromise: resp[1].projectPromise
      };
      angular.extend($scope, context);
      // FIXME: DataService.createStream() requires a scope with a
      // projectPromise rather than just a namespace, so we have to pass the
      // context into the log-viewer directive.
      $scope.logContext = context;
      DataService.get("pods", $routeParams.pod, $scope).then(
        // success
        function(pod) {
          $scope.loaded = true;
          $scope.pod = pod;
          $scope.logOptions.container = $routeParams.container || pod.spec.containers[0].name;
          var pods = {};
          pods[pod.metadata.name] = pod;
          ImageStreamResolver.fetchReferencedImageStreamImages(pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);

          // If we found the item successfully, watch for changes on it
          watches.push(DataService.watchObject("pods", $routeParams.pod, $scope, function(pod, action) {
            if (action === "DELETED") {
              $scope.alerts["deleted"] = {
                type: "warning",
                message: "This pod has been deleted."
              };
            }
            $scope.pod = pod;
          }));
        },
        // failure
        function(e) {
          $scope.loaded = true;
          $scope.alerts["load"] = {
            type: "error",
            message: "The pod details could not be loaded.",
            details: "Reason: " + $filter('getErrorDetails')(e)
          };
        }
      );

      // Sets up subscription for imageStreams
      watches.push(DataService.watch("imagestreams", $scope, function(imageStreams) {
        $scope.imageStreams = imageStreams.by("metadata.name");
        ImageStreamResolver.buildDockerRefMapForImageStreams($scope.imageStreams, $scope.imageStreamImageRefByDockerReference);
        ImageStreamResolver.fetchReferencedImageStreamImages($scope.pods, $scope.imagesByDockerReference, $scope.imageStreamImageRefByDockerReference, $scope);
        Logger.log("imagestreams (subscribe)", $scope.imageStreams);
      }));

      watches.push(DataService.watch("builds", $scope, function(builds) {
        $scope.builds = builds.by("metadata.name");
        Logger.log("builds (subscribe)", $scope.builds);
      }));
    });

    $scope.containersRunning = function(containerStatuses) {
      var running = 0;
      if (containerStatuses) {
        containerStatuses.forEach(function(v) {
          if (v.state && v.state.running) {
            running++;
          }
        });
      }
      return running;
    };

    $scope.$on('$destroy', function(){
      DataService.unwatchAll(watches);
    });
  });
