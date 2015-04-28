'use strict';

angular.module('openshiftConsole')
  // this filter is intended for use with the "track by" in an ng-repeat
  // when uid is not defined it falls back to object identity for uniqueness
  .filter('uid', function() {
    return function(resource) {
      if (resource && resource.metadata && resource.metadata.uid) {
        return resource.metadata.uid;
      }
      else {
        return resource;
      }
    }
  })
  .filter('annotation', function() {
    return function(resource, key) {
      if (resource && resource.spec && resource.spec.tags && key.indexOf(".") !== -1){
        var tagAndKey = key.split(".");
        var tags = resource.spec.tags;
        for(var i=0; i < tags.length; ++i){
          var tag = tags[i];
          var tagName = tagAndKey[0];
          var tagKey = tagAndKey[1];
          if(tagName === tag.name && tag.annotations){
            return tag.annotations[tagKey];
          }
        }
      }
      if (resource && resource.metadata && resource.metadata.annotations) {
        return resource.metadata.annotations[key];
      }
      return null;
    };
  })
  .filter('description', function(annotationFilter) {
    return function(resource) {
      return annotationFilter(resource, "description");
    };
  })
  .filter('tags', function(annotationFilter) {
    return function(resource, annotationKey) {
      annotationKey = annotationKey || "tags";
      var tags = annotationFilter(resource, annotationKey);
      if (!tags) {
        return [];
      }
      return tags.split(/\s*,\s*/);
    };
  })
  .filter('label', function() {
    return function(resource, key) {
      if (resource && resource.metadata && resource.metadata.labels) {
        return resource.metadata.labels[key];
      }
      return null;
    };
  })
  .filter('icon', function(annotationFilter) {
    return function(resource) {
      var icon = annotationFilter(resource, "icon");
      if (!icon) {
        //FIXME: Return default icon for resource.kind
        return "";
      } else {
        return icon;
      }
    };
  })
  .filter('iconClass', function(annotationFilter) {
    return function(resource, kind, annotationKey) {
      annotationKey = annotationKey || "iconClass";
      var icon = annotationFilter(resource, annotationKey);
      if (!icon) {
        if (kind === "template") {
          return "fa fa-bolt";
        }
        if (kind === "image") {
          return "fa fa-cube";
        }        
        else {
          return "";
        }
      }
      else {
        return icon;
      }
    };
  })
  .filter('imageName', function() {
    return function(image) {
      if (!image) {
        return "";
      }
      // TODO move this parsing method into a utility method
      var slashSplit = image.split("/");
      var semiColonSplit;
      if (slashSplit.length === 3) {
        semiColonSplit = slashSplit[2].split(":");
        return slashSplit[1] + '/' + semiColonSplit[0];
      }
      else if (slashSplit.length === 2) {
        // TODO umm tough... this could be registry/imageName or imageRepo/imageName
        // have to check if the first bit matches a registry pattern, will handle this later...
        return image;
      }
      else if (slashSplit.length === 1) {
        semiColonSplit = image.split(":");
        return semiColonSplit[0];
      }
    };
  })
  .filter('imageEnv', function() {
    return function(image, envKey) {
      var envVars = image.dockerImageMetadata.Config.Env;
      for (var i = 0; i < envVars.length; i++) {
        var keyValue = envVars[i].split("=");
        if (keyValue[0] === envKey) {
          return keyValue[1];
        }
      }
      return null;
    };
  })  
  .filter('buildForImage', function() {
    return function(image, builds) {
      // TODO concerned that this gets called anytime any data is changed on the scope, whether its relevant changes or not
      var envVars = image.dockerImageMetadata.Config.Env;
      for (var i = 0; i < envVars.length; i++) {
        var keyValue = envVars[i].split("=");
        if (keyValue[0] === "OPENSHIFT_BUILD_NAME") {
          return builds[keyValue[1]];
        }
      }
      return null;
    };
  })
  .filter('webhookURL', function(DataService) {
    return function(buildConfig, type, secret, project) {
      return DataService.url({
        type: "buildConfigHooks",
        id: buildConfig,
        namespace: project,
        secret: secret,
        hookType: type,
      });
    };
  })
  .filter('isWebRoute', function(){
    return function(route){
       //TODO: implement when we can tell if routes are http(s) or not web related which will drive links in view
       return true;
    };
  })
  .filter('routeWebURL', function(){
    return function(route){
        var scheme = (route.tls && route.tls.tlsTerminationType !== "") ? "https" : "http";
        var url = scheme + "://" + route.host;
        if (route.path) {
            url += route.path;
        }
        return url;
    };
  })
  .filter('parameterPlaceholder', function() {
    return function(parameter) {
      if (parameter.generate) {
        return "(generated if empty)";
      } else {
        return "";
      }
    };
  })
 .filter('parameterValue', function() {
    return function(parameter) {
      if (!parameter.value && parameter.generate) {
        return "(generated)";
      } else {
        return parameter.value;
      }
    };
  })
  .filter('provider', function() {
    return function(resource) {
      return (resource && resource.annotations && resource.annotations.provider) ||
        (resource && resource.metadata && resource.metadata.namespace);
    };
  })
  .filter('imageRepoReference', function(){
    return function(objectRef, tag){
      tag = tag || "latest";
      var ns = objectRef.namespace || "";
      ns = ns == "" ? ns : ns + "/";
      var ref = ns + objectRef.name;
      ref += " [" + tag + "]";
      return ref;
    };
  });
