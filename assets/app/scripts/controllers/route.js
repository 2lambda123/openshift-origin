'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:ServiceController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('RouteController', function ($scope, $routeParams, DataService, ProjectsService, $filter) {
    $scope.projectName = $routeParams.project;
    $scope.route = null;
    $scope.alerts = {};
    $scope.renderOptions = {
      hideFilterWidget: true
    };
    $scope.breadcrumbs = [
      {
        title: "Routes",
        link: "project/" + $routeParams.project + "/browse/routes"
      },
      {
        title: $routeParams.route
      }
    ];

    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;
        DataService.get("routes", $routeParams.route, context).then(
          // success
          function(route) {
            $scope.loaded = true;
            $scope.route = route;

            // If we found the item successfully, watch for changes on it
            watches.push(DataService.watchObject("routes", $routeParams.route, context, function(route, action) {
              if (action === "DELETED") {
                $scope.alerts["deleted"] = {
                  type: "warning",
                  message: "This route has been deleted."
                };
              }
              $scope.route = route;
            }));
          },
          // failure
          function(e) {
            $scope.loaded = true;
            $scope.alerts["load"] = {
              type: "error",
              message: "The route details could not be loaded.",
              details: "Reason: " + $filter('getErrorDetails')(e)
            };
          });
        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });
    }));
  });
