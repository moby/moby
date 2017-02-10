#include <string.h>
#include <stdlib.h>
#include "secretservice_linux.h"

const SecretSchema *docker_get_schema(void)
{
	static const SecretSchema docker_schema = {
		"io.docker.Credentials", SECRET_SCHEMA_NONE,
		{
			{ "server", SECRET_SCHEMA_ATTRIBUTE_STRING },
			{ "username", SECRET_SCHEMA_ATTRIBUTE_STRING },
			{ "docker_cli", SECRET_SCHEMA_ATTRIBUTE_STRING },
			{ "NULL", 0 },
		}
	};
	return &docker_schema;
}

GError *add(char *server, char *username, char *secret) {
	GError *err = NULL;

	secret_password_store_sync (DOCKER_SCHEMA, SECRET_COLLECTION_DEFAULT,
			server, secret, NULL, &err,
			"server", server,
			"username", username,
			"docker_cli", "1",
			NULL);
	return err;
}

GError *delete(char *server) {
	GError *err = NULL;

	secret_password_clear_sync(DOCKER_SCHEMA, NULL, &err,
			"server", server,
			"docker_cli", "1",
			NULL);
	if (err != NULL)
		return err;
	return NULL;
}

char *get_username(SecretItem *item) {
	GHashTable *attributes;
	GHashTableIter iter;
	gchar *value, *key;

	attributes = secret_item_get_attributes(item);
	g_hash_table_iter_init(&iter, attributes);
	while (g_hash_table_iter_next(&iter, (void **)&key, (void **)&value)) {
		if (strncmp(key, "username", strlen(key)) == 0)
			return (char *)value;
	}
	g_hash_table_unref(attributes);
	return NULL;
}

GError *get(char *server, char **username, char **secret) {
	GError *err = NULL;
	GHashTable *attributes;
	SecretService *service;
	GList *items, *l;
	SecretSearchFlags flags = SECRET_SEARCH_LOAD_SECRETS | SECRET_SEARCH_ALL | SECRET_SEARCH_UNLOCK;
	SecretValue *secretValue;
	gsize length;
	gchar *value;

	attributes = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_free);
	g_hash_table_insert(attributes, g_strdup("server"), g_strdup(server));
	g_hash_table_insert(attributes, g_strdup("docker_cli"), g_strdup("1"));

	service = secret_service_get_sync(SECRET_SERVICE_NONE, NULL, &err);
	if (err == NULL) {
		items = secret_service_search_sync(service, NULL, attributes, flags, NULL, &err);
		if (err == NULL) {
			for (l = items; l != NULL; l = g_list_next(l)) {
				value = secret_item_get_schema_name(l->data);
				if (strncmp(value, "io.docker.Credentials", strlen(value)) != 0) {
					g_free(value);
					continue;
				}
				g_free(value);
				secretValue = secret_item_get_secret(l->data);
				if (secret != NULL) {
					*secret = strdup(secret_value_get(secretValue, &length));
					secret_value_unref(secretValue);
				}
				*username = get_username(l->data);
			}
			g_list_free_full(items, g_object_unref);
		}
		g_object_unref(service);
	}
	g_hash_table_unref(attributes);
	if (err != NULL) {
		return err;
	}
	return NULL;
}

GError *list(char *** paths, char *** accts, unsigned int *list_l) {
	GList *items;
	GError *err = NULL;
	SecretService *service;
	SecretSearchFlags flags = SECRET_SEARCH_LOAD_SECRETS | SECRET_SEARCH_ALL | SECRET_SEARCH_UNLOCK;
	GHashTable *attributes;
	g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_free);
	attributes = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_free);
	service = secret_service_get_sync(SECRET_SERVICE_NONE, NULL, &err);
	items = secret_service_search_sync(service, NULL, attributes, flags, NULL, &err);
	int numKeys = g_list_length(items);
	if (err != NULL) {
		return err;
	}
	*paths = (char **) malloc((int)sizeof(char *)*numKeys);
	*accts = (char **) malloc((int)sizeof(char *)*numKeys);
	// items now contains our keys from the gnome keyring
	// we will now put it in our two lists to return it to go
	GList *current;
	int listNumber = 0;
	for(current = items; current!=NULL; current = current->next) {
		char *pathTmp = secret_item_get_label(current->data);
		// you cannot have a key without a label in the gnome keyring
		char *acctTmp = get_username(current->data);
		if (acctTmp==NULL) {
			acctTmp = "account not defined";
		}
		char *path = (char *) malloc(strlen(pathTmp));
		char *acct = (char *) malloc(strlen(acctTmp));
		path = pathTmp;
		acct = acctTmp;
		(*paths)[listNumber] = (char *) malloc(sizeof(char)*(strlen(path)));
		memcpy((*paths)[listNumber], path, sizeof(char)*(strlen(path)));
		(*accts)[listNumber] = (char *) malloc(sizeof(char)*(strlen(acct)));
		memcpy((*accts)[listNumber], acct, sizeof(char)*(strlen(acct)));
		listNumber = listNumber + 1;
	}
	*list_l = numKeys;
	return NULL;
}

void freeListData(char *** data, unsigned int length) {
	int i;
	for(i=0; i<length; i++) {
		free((*data)[i]);
	}
	free(*data);
}
