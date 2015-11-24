'use strict';

/**
 * @ngdoc function
 * @name openshiftConsole.controller:BuildConfigController
 * @description
 * Controller of the openshiftConsole
 */
angular.module('openshiftConsole')
  .controller('BuildConfigController', function ($scope, $routeParams, DataService, BuildsService, ProjectsService, $filter, LabelFilter) {
    $scope.projectName = $routeParams.project;
    $scope.buildConfig = null;
    $scope.builds = {};
    $scope.unfilteredBuilds = {};
    $scope.labelSuggestions = {};
    $scope.alerts = {};
    $scope.breadcrumbs = [
      {
        title: "Builds",
        link: "project/" + $routeParams.project + "/browse/builds"
      },
      {
        title: $routeParams.buildconfig
      }
    ];
    $scope.emptyMessage = "Loading...";

    var watches = [];

    ProjectsService
      .get($routeParams.project)
      .then(_.spread(function(project, context) {
        $scope.project = project;

        DataService
          .get("buildconfigs", $routeParams.buildconfig, context)
          .then(function(buildConfig) {

            $scope.loaded = true;
            $scope.buildConfig = buildConfig;

            // If we found the item successfully, watch for changes on it
            watches.push(DataService.watchObject("buildconfigs", $routeParams.buildconfig, context, function(buildConfig, action) {
              if (action === "DELETED") {
                $scope.alerts["deleted"] = {
                  type: "warning",
                  message: "This build configuration has been deleted."
                };
              }
              $scope.buildConfig = buildConfig;
            }));

          }, function(e) {
            $scope.loaded = true;
            $scope.alerts["load"] = {
              type: "error",
              message: "The build configuration details could not be loaded.",
              details: "Reason: " + $filter('getErrorDetails')(e)
            };
          });

        watches.push(DataService.watch("builds", context, function(builds, action, build) {
          $scope.emptyMessage = "No builds to show";
          // TODO we should send the ?labelSelector=buildconfig=<name> on the API request
          // to only load the buildconfig's builds, but this requires some DataService changes

          if (!action) {
            $scope.unfilteredBuilds = {};
            var allBuilds = builds.by("metadata.name");
            angular.forEach(allBuilds, function(build, name) {
              if (build.metadata.labels && build.metadata.labels.buildconfig === $routeParams.buildconfig) {
                $scope.unfilteredBuilds[name] = build;
              }
            });

            // Loading of the page that will create buildConfigBuildsInProgress structure, which will associate running build to his buildConfig.
            $scope.buildConfigBuildsInProgress = BuildsService.associateRunningBuildToBuildConfig($scope.unfilteredBuilds);
          } else if (build.metadata.labels && build.metadata.labels.buildconfig === $routeParams.buildconfig) {
            var buildName = build.metadata.name;
            var buildConfigName = $routeParams.buildconfig;
            switch (action) {
              case 'ADDED':
              case 'MODIFIED':
                $scope.unfilteredBuilds[buildName] = build;
                // After the build ends remove him from the buildConfigBuildsInProgress structure.
                if ($filter('isIncompleteBuild')(build)){
                  $scope.buildConfigBuildsInProgress[buildConfigName] = $scope.buildConfigBuildsInProgress[buildConfigName] || {};
                  $scope.buildConfigBuildsInProgress[buildConfigName][buildName] = build;
                } else if ($scope.buildConfigBuildsInProgress[buildConfigName]) {
                  delete $scope.buildConfigBuildsInProgress[buildConfigName][buildName];
                }
                break;
              case 'DELETED':
                delete $scope.unfilteredBuilds[buildName];
                if ($scope.buildConfigBuildsInProgress[buildConfigName]){
                  delete $scope.buildConfigBuildsInProgress[buildConfigName][buildName];
                }
                break;
            }
          }

          $scope.builds = LabelFilter.getLabelSelector().select($scope.unfilteredBuilds);
          updateFilterWarning();
          LabelFilter.addLabelSuggestionsFromResources($scope.unfilteredBuilds, $scope.labelSuggestions);
          LabelFilter.setLabelSuggestions($scope.labelSuggestions);
        }));

        function updateFilterWarning() {
          if (!LabelFilter.getLabelSelector().isEmpty() && $.isEmptyObject($scope.builds) && !$.isEmptyObject($scope.unfilteredBuilds)) {
            $scope.alerts["builds"] = {
              type: "warning",
              details: "The active filters are hiding all builds."
            };
          }
          else {
            delete $scope.alerts["builds"];
          }
        }

        LabelFilter.onActiveFiltersChanged(function(labelSelector) {
          // trigger a digest loop
          $scope.$apply(function() {
            $scope.builds = labelSelector.select($scope.unfilteredBuilds);
            updateFilterWarning();
          });
        });

       $scope.startBuild = function(buildConfigName) {
          BuildsService
            .startBuild(buildConfigName, context)
            .then(function(build) { //success
              $scope.alerts["create"] = {
                type: "success",
                message: "Build " + build.metadata.name + " has started."
              };
            }, function(result) { //failure
              $scope.alerts["create"] = {
                type: "error",
                message: "An error occurred while starting the build.",
                details: $filter('getErrorDetails')(result)
              };
            });
        };

        $scope.cancelBuild = function(build, buildConfigName) {
          BuildsService
            .cancelBuild(build, buildConfigName, context)
            .then(function() {
              $scope.alerts["cancel"] = {
                type: "success",
                message: "Cancelling build " + build.metadata.name + " of " + buildConfigName + "."
              };
            }, function(result) {
              $scope.alerts["cancel"] = {
                type: "error",
                message: "An error occurred cancelling the build.",
                details: $filter('getErrorDetails')(result)
              };
            });
        };

        $scope.cloneBuild = function(buildName) {
          BuildsService
            .cloneBuild(buildName, context)
            .then(function(build) {
                $scope.alerts["rebuild"] = {
                  type: "success",
                  message: "Build " + buildName + " is being rebuilt as " + build.metadata.name + "."
                };
            }, function(result) {
              $scope.alerts["rebuild"] = {
                type: "error",
                message: "An error occurred while rerunning the build.",
                details: $filter('getErrorDetails')(result)
              };
            });
        };

        $scope.$on('$destroy', function(){
          DataService.unwatchAll(watches);
        });

      }));
  });
