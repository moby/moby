#include "osxkeychain_darwin.h"
#include <CoreFoundation/CoreFoundation.h>
#include <stdio.h>
#include <string.h>

char *get_error(OSStatus status) {
  char *buf = malloc(128);
  CFStringRef str = SecCopyErrorMessageString(status, NULL);
  int success = CFStringGetCString(str, buf, 128, kCFStringEncodingUTF8);
  if (!success) {
    strncpy(buf, "Unknown error", 128);
  }
  return buf;
}

char *keychain_add(struct Server *server, char *username, char *secret) {
  OSStatus status = SecKeychainAddInternetPassword(
    NULL,
    strlen(server->host), server->host,
    0, NULL,
    strlen(username), username,
    strlen(server->path), server->path,
    server->port,
    server->proto,
    kSecAuthenticationTypeDefault,
    strlen(secret), secret,
    NULL
  );
  if (status) {
    return get_error(status);
  }
  return NULL;
}

char *keychain_get(struct Server *server, unsigned int *username_l, char **username, unsigned int *secret_l, char **secret) {
  char *tmp;
  SecKeychainItemRef item;

  OSStatus status = SecKeychainFindInternetPassword(
    NULL,
    strlen(server->host), server->host,
    0, NULL,
    0, NULL,
    strlen(server->path), server->path,
    server->port,
    server->proto,
    kSecAuthenticationTypeDefault,
    secret_l, (void **)&tmp,
    &item);

  if (status) {
    return get_error(status);
  }

  *secret = strdup(tmp);
  SecKeychainItemFreeContent(NULL, tmp);

  SecKeychainAttributeList list;
  SecKeychainAttribute attr;

  list.count = 1;
  list.attr = &attr;
  attr.tag = kSecAccountItemAttr;

  status = SecKeychainItemCopyContent(item, NULL, &list, NULL, NULL);
  if (status) {
    return get_error(status);
  }

  *username = strdup(attr.data);
  *username_l = attr.length;
  SecKeychainItemFreeContent(&list, NULL);

  return NULL;
}

char *keychain_delete(struct Server *server) {
  SecKeychainItemRef item;

  OSStatus status = SecKeychainFindInternetPassword(
    NULL,
    strlen(server->host), server->host,
    0, NULL,
    0, NULL,
    strlen(server->path), server->path,
    server->port,
    server->proto,
    kSecAuthenticationTypeDefault,
    0, NULL,
    &item);

  if (status) {
    return get_error(status);
  }

  status = SecKeychainItemDelete(item);
  if (status) {
    return get_error(status);
  }
  return NULL;
}

char * CFStringToCharArr(CFStringRef aString) {
  if (aString == NULL) {
    return NULL;
  }
  CFIndex length = CFStringGetLength(aString);
  CFIndex maxSize =
  CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
  char *buffer = (char *)malloc(maxSize);
  if (CFStringGetCString(aString, buffer, maxSize,
                         kCFStringEncodingUTF8)) {
    return buffer;
  }
  return NULL;
}

char *keychain_list(char *** paths, char *** accts, unsigned int *list_l) {
    CFMutableDictionaryRef query = CFDictionaryCreateMutable (NULL, 1, NULL, NULL);
    CFDictionaryAddValue(query, kSecClass, kSecClassInternetPassword);
    CFDictionaryAddValue(query, kSecReturnAttributes, kCFBooleanTrue);
    CFDictionaryAddValue(query, kSecMatchLimit, kSecMatchLimitAll);
    //Use this query dictionary
    CFTypeRef result= NULL;
    OSStatus status = SecItemCopyMatching(
    query,
    &result);
    //Ran a search and store the results in result
    if (status) {
        return get_error(status);
    }
    int numKeys = CFArrayGetCount(result);
    *paths = (char **) malloc((int)sizeof(char *)*numKeys);
    *accts = (char **) malloc((int)sizeof(char *)*numKeys);
    //result is of type CFArray
    for(int i=0; i<numKeys; i++) {
        CFDictionaryRef currKey = CFArrayGetValueAtIndex(result,i);
        if (CFDictionaryContainsKey(currKey, CFSTR("path"))) {
            //Even if a key is stored without an account, Apple defaults it to null so these arrays will be of the same length
            CFStringRef pathTmp = CFDictionaryGetValue(currKey, CFSTR("path"));
            CFStringRef acctTmp = CFDictionaryGetValue(currKey, CFSTR("acct"));
            if (acctTmp == NULL) {
                acctTmp = CFSTR("account not defined");
            }
            char * path = (char *) malloc(CFStringGetLength(pathTmp)+1);
            path = CFStringToCharArr(pathTmp);
            path[strlen(path)] = '\0';
            char * acct = (char *) malloc(CFStringGetLength(acctTmp)+1);
            acct = CFStringToCharArr(acctTmp);
            acct[strlen(acct)] = '\0';
            //We now have all we need, username and servername. Now export this to .go
            (*paths)[i] = (char *) malloc(sizeof(char)*(strlen(path)+1));
            memcpy((*paths)[i], path, sizeof(char)*(strlen(path)+1));
            (*accts)[i] = (char *) malloc(sizeof(char)*(strlen(acct)+1));
            memcpy((*accts)[i], acct, sizeof(char)*(strlen(acct)+1));
        }
        else {
            char * path = "0";
            char * acct = "0";
            (*paths)[i] = (char *) malloc(sizeof(char)*(strlen(path)));
            memcpy((*paths)[i], path, sizeof(char)*(strlen(path)));
            (*accts)[i] = (char *) malloc(sizeof(char)*(strlen(acct)));
            memcpy((*accts)[i], acct, sizeof(char)*(strlen(acct)));
        }
    }
    *list_l = numKeys;
    return NULL;
}

void freeListData(char *** data, unsigned int length) {
     for(int i=0; i<length; i++) {
        free((*data)[i]);
     }
     free(*data);
}
