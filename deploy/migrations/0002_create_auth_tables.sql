CREATE TABLE IF NOT EXISTS auth_users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(128) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    disabled TINYINT(1) NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_roles (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(128) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_permissions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(128) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_user_roles (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    assigned_at BIGINT NOT NULL,
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_auth_user_roles_user FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE,
    CONSTRAINT fk_auth_user_roles_role FOREIGN KEY (role_id) REFERENCES auth_roles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS auth_role_permissions (
    role_id BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    assigned_at BIGINT NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    CONSTRAINT fk_auth_role_permissions_role FOREIGN KEY (role_id) REFERENCES auth_roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_auth_role_permissions_permission FOREIGN KEY (permission_id) REFERENCES auth_permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS auth_user_permissions (
    user_id BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    assigned_at BIGINT NOT NULL,
    PRIMARY KEY (user_id, permission_id),
    CONSTRAINT fk_auth_user_permissions_user FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE,
    CONSTRAINT fk_auth_user_permissions_permission FOREIGN KEY (permission_id) REFERENCES auth_permissions(id) ON DELETE CASCADE
);
